// Copyright (c) 2016-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package sqlstore

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost-server/model"
	"github.com/mattermost/mattermost-server/store"
)

type SqlComplianceStore struct {
	SqlStore
}

func NewSqlComplianceStore(sqlStore SqlStore) store.ComplianceStore {
	s := &SqlComplianceStore{sqlStore}

	for _, db := range sqlStore.GetAllConns() {
		table := db.AddTableWithName(model.Compliance{}, "Compliances").SetKeys(false, "Id")
		table.ColMap("Id").SetMaxSize(26)
		table.ColMap("UserId").SetMaxSize(26)
		table.ColMap("Status").SetMaxSize(64)
		table.ColMap("Desc").SetMaxSize(512)
		table.ColMap("Type").SetMaxSize(64)
		table.ColMap("Keywords").SetMaxSize(512)
		table.ColMap("Emails").SetMaxSize(1024)
	}

	return s
}

func (s SqlComplianceStore) CreateIndexesIfNotExists() {
}

func (s SqlComplianceStore) Save(compliance *model.Compliance) store.StoreChannel {
	return store.Do(func(result *store.StoreResult) {
		compliance.PreSave()
		if result.Err = compliance.IsValid(); result.Err != nil {
			return
		}

		if err := s.GetMaster().Insert(compliance); err != nil {
			result.Err = model.NewAppError("SqlComplianceStore.Save", "store.sql_compliance.save.saving.app_error", nil, err.Error(), http.StatusInternalServerError)
		} else {
			result.Data = compliance
		}
	})
}

func (us SqlComplianceStore) Update(compliance *model.Compliance) store.StoreChannel {
	return store.Do(func(result *store.StoreResult) {
		if result.Err = compliance.IsValid(); result.Err != nil {
			return
		}

		if _, err := us.GetMaster().Update(compliance); err != nil {
			result.Err = model.NewAppError("SqlComplianceStore.Update", "store.sql_compliance.save.saving.app_error", nil, err.Error(), http.StatusInternalServerError)
		} else {
			result.Data = compliance
		}
	})
}

func (s SqlComplianceStore) GetAll(offset, limit int) store.StoreChannel {
	return store.Do(func(result *store.StoreResult) {
		query := "SELECT * FROM Compliances ORDER BY CreateAt DESC LIMIT :Limit OFFSET :Offset"

		var compliances model.Compliances
		if _, err := s.GetReplica().Select(&compliances, query, map[string]interface{}{"Offset": offset, "Limit": limit}); err != nil {
			result.Err = model.NewAppError("SqlComplianceStore.Get", "store.sql_compliance.get.finding.app_error", nil, err.Error(), http.StatusInternalServerError)
		} else {
			result.Data = compliances
		}
	})
}

func (us SqlComplianceStore) Get(id string) store.StoreChannel {
	return store.Do(func(result *store.StoreResult) {
		if obj, err := us.GetReplica().Get(model.Compliance{}, id); err != nil {
			result.Err = model.NewAppError("SqlComplianceStore.Get", "store.sql_compliance.get.finding.app_error", nil, err.Error(), http.StatusInternalServerError)
		} else if obj == nil {
			result.Err = model.NewAppError("SqlComplianceStore.Get", "store.sql_compliance.get.finding.app_error", nil, err.Error(), http.StatusNotFound)
		} else {
			result.Data = obj.(*model.Compliance)
		}
	})
}

func (s SqlComplianceStore) ComplianceExport(job *model.Compliance) store.StoreChannel {
	return store.Do(func(result *store.StoreResult) {
		props := map[string]interface{}{"StartTime": job.StartAt, "EndTime": job.EndAt}

		keywordQuery := ""
		keywords := strings.Fields(strings.TrimSpace(strings.ToLower(strings.Replace(job.Keywords, ",", " ", -1))))
		if len(keywords) > 0 {

			keywordQuery = "AND ("

			for index, keyword := range keywords {
				if index >= 1 {
					keywordQuery += " OR LOWER(Posts.Message) LIKE :Keyword" + strconv.Itoa(index)
				} else {
					keywordQuery += "LOWER(Posts.Message) LIKE :Keyword" + strconv.Itoa(index)
				}

				props["Keyword"+strconv.Itoa(index)] = "%" + keyword + "%"
			}

			keywordQuery += ")"
		}

		emailQuery := ""
		emails := strings.Fields(strings.TrimSpace(strings.ToLower(strings.Replace(job.Emails, ",", " ", -1))))
		if len(emails) > 0 {

			emailQuery = "AND ("

			for index, email := range emails {
				if index >= 1 {
					emailQuery += " OR Users.Email = :Email" + strconv.Itoa(index)
				} else {
					emailQuery += "Users.Email = :Email" + strconv.Itoa(index)
				}

				props["Email"+strconv.Itoa(index)] = email
			}

			emailQuery += ")"
		}

		query :=
			`(SELECT
			    Teams.Name AS TeamName,
			    Teams.DisplayName AS TeamDisplayName,
			    Channels.Name AS ChannelName,
			    Channels.DisplayName AS ChannelDisplayName,
			    Users.Username AS UserUsername,
			    Users.Email AS UserEmail,
			    Users.Nickname AS UserNickname,
			    Posts.Id AS PostId,
			    Posts.CreateAt AS PostCreateAt,
			    Posts.UpdateAt AS PostUpdateAt,
			    Posts.DeleteAt AS PostDeleteAt,
			    Posts.RootId AS PostRootId,
			    Posts.ParentId AS PostParentId,
			    Posts.OriginalId AS PostOriginalId,
			    Posts.Message AS PostMessage,
			    Posts.Type AS PostType,
			    Posts.Props AS PostProps,
			    Posts.Hashtags AS PostHashtags,
			    Posts.FileIds AS PostFileIds
			FROM
			    Teams,
			    Channels,
			    Users,
			    Posts
			WHERE
			    Teams.Id = Channels.TeamId
			        AND Posts.ChannelId = Channels.Id
			        AND Posts.UserId = Users.Id
			        AND Posts.CreateAt > :StartTime
			        AND Posts.CreateAt <= :EndTime
			        ` + emailQuery + `
			        ` + keywordQuery + `)
			UNION ALL
			(SELECT
			    'direct-messages' AS TeamName,
			    'Direct Messages' AS TeamDisplayName,
			    Channels.Name AS ChannelName,
			    Channels.DisplayName AS ChannelDisplayName,
			    Users.Username AS UserUsername,
			    Users.Email AS UserEmail,
			    Users.Nickname AS UserNickname,
			    Posts.Id AS PostId,
			    Posts.CreateAt AS PostCreateAt,
			    Posts.UpdateAt AS PostUpdateAt,
			    Posts.DeleteAt AS PostDeleteAt,
			    Posts.RootId AS PostRootId,
			    Posts.ParentId AS PostParentId,
			    Posts.OriginalId AS PostOriginalId,
			    Posts.Message AS PostMessage,
			    Posts.Type AS PostType,
			    Posts.Props AS PostProps,
			    Posts.Hashtags AS PostHashtags,
			    Posts.FileIds AS PostFileIds
			FROM
			    Channels,
			    Users,
			    Posts
			WHERE
			    Channels.TeamId = ''
			        AND Posts.ChannelId = Channels.Id
			        AND Posts.UserId = Users.Id
			        AND Posts.CreateAt > :StartTime
			        AND Posts.CreateAt <= :EndTime
			        ` + emailQuery + `
			        ` + keywordQuery + `)
			ORDER BY PostCreateAt
			LIMIT 30000`

		var cposts []*model.CompliancePost

		if _, err := s.GetReplica().Select(&cposts, query, props); err != nil {
			result.Err = model.NewAppError("SqlPostStore.ComplianceExport", "store.sql_post.compliance_export.app_error", nil, err.Error(), http.StatusInternalServerError)
		} else {
			result.Data = cposts
		}
	})
}

func (s SqlComplianceStore) MessageExport(after int64, limit int64) store.StoreChannel {
	storeChannel := make(store.StoreChannel, 1)

	go func() {
		props := map[string]interface{}{"StartTime": after, "Limit": limit}

		queryPrefix :=
			`SELECT
				Posts.Id AS PostId,
				Posts.CreateAt AS PostCreateAt,
				Posts.UpdateAt AS PostUpdateAt,
				Posts.DeleteAt AS PostDeleteAt,
				Posts.RootId AS PostRootId,
				Posts.ParentId AS PostParentId,
				Posts.OriginalId AS PostOriginalId,
				Posts.Message AS PostMessage,
				Posts.Type AS PostType,
				Posts.Props AS PostProps,
				Posts.Hashtags AS PostHashtags,
				Posts.FileIds AS PostFileIds,
				Channels.Id AS ChannelId,
				Channels.CreateAt AS ChannelCreateAt,
				Channels.UpdateAt AS ChannelUpdateAt,
				Channels.DeleteAt AS ChannelDeleteAt,
				Channels.DisplayName AS ChannelDisplayName,
				Channels.Name AS ChannelName,
				Channels.Header AS ChannelHeader,
				Channels.Purpose AS ChannelPurpose,
				Channels.LastPostAt AS ChannelLastPostAt,
				Users.Id AS UserId,
				Users.CreateAt AS UserCreateAt,
				Users.UpdateAt AS UserUpdateAt,
				Users.DeleteAt AS UserDeleteAt,
				Users.Username AS UserUsername,
				Users.Email AS UserEmail,
				Users.Nickname AS UserNickname,
				Users.FirstName AS UserFirstName,
				Users.LastName AS UserLastName,
				Teams.Id AS TeamId,
				Teams.CreateAt AS TeamCreateAt,
				Teams.UpdateAt AS TeamUpdateAt,
				Teams.DeleteAt AS TeamDeleteAt,
				Teams.DisplayName AS TeamDisplayName,
				Teams.Name AS TeamName,
				Teams.Description AS TeamDescription,
				Teams.AllowOpenInvite AS TeamAllowOpenInvite, `

		queryMySql :=
			`CASE
				WHEN Posts.Type = 'system_add_to_channel'
				THEN (SELECT Email FROM Users WHERE Username = JSON_UNQUOTE(JSON_EXTRACT(CAST(Posts.Props AS JSON), '$.addedUsername')))
				ELSE NULL END AS AddedUserEmail,
			CASE
				WHEN Posts.Type = 'system_remove_from_channel'
				THEN (SELECT Email FROM Users WHERE Username = JSON_UNQUOTE(JSON_EXTRACT(CAST(Posts.Props AS JSON), '$.removedUsername')))
				ELSE NULL END AS RemovedUserEmail `

		queryPostgres :=
			`CASE
				WHEN Posts.Type = 'system_add_to_channel'
				THEN (SELECT Email FROM Users WHERE Username = JSON_EXTRACT_PATH_TEXT(CAST(Posts.Props AS json), 'addedUsername'))
				ELSE NULL END AS AddedUserEmail,
			CASE
				WHEN Posts.Type = 'system_remove_from_channel'
				THEN (SELECT Email FROM Users WHERE Username = JSON_EXTRACT_PATH_TEXT(CAST(Posts.Props AS json), 'removedUsername'))
				ELSE NULL END AS RemovedUserEmail `

		querySuffix :=
			`FROM
				Posts
				LEFT OUTER JOIN Channels ON Posts.ChannelId = Channels.Id
				LEFT OUTER JOIN Users ON Posts.UserId = Users.Id
				LEFT OUTER JOIN Teams ON Channels.TeamId = Teams.Id
			WHERE
				Posts.CreateAt > :StartTime
			ORDER BY PostCreateAt
			LIMIT :Limit`

		var cposts []*model.MessageExport
		result := store.StoreResult{}

		query := queryPrefix
		if s.DriverName() != model.DATABASE_DRIVER_MYSQL && s.DriverName() != model.DATABASE_DRIVER_POSTGRES {
			result.Err = model.NewAppError("SqlPreferenceStore.save", "store.sql_preference.save.missing_driver.app_error", nil, "Failed to update preference because of missing driver", http.StatusNotImplemented)
		} else {
			if s.DriverName() == model.DATABASE_DRIVER_MYSQL {
				query += queryMySql
			} else if s.DriverName() == model.DATABASE_DRIVER_POSTGRES {
				query += queryPostgres
			}
			query += querySuffix

			if _, err := s.GetReplica().Select(&cposts, query, props); err != nil {
				result.Err = model.NewAppError("SqlComplianceStore.MessageExport", "store.sql_compliance.message_export.app_error", nil, err.Error(), http.StatusInternalServerError)
			} else {
				result.Data = cposts
			}
		}

		storeChannel <- result
		close(storeChannel)
	}()

	return storeChannel
}