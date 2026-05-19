package banter

import "sort"

const (
	PermSendMessages     int64 = 1 << 0
	PermManageChannels   int64 = 1 << 1
	PermManageRoles      int64 = 1 << 2
	PermManageMessages   int64 = 1 << 3
	PermAdministrator    int64 = 1 << 4
	PermMentionEveryone  int64 = 1 << 5
	PermViewChannels     int64 = 1 << 6
	PermAttachFiles      int64 = 1 << 7
	PermBanMembers       int64 = 1 << 8
	PermUseSlashCommands int64 = 1 << 9
	PermManageGuild      int64 = 1 << 10
	PermKickMembers      int64 = 1 << 11
)

func PermsAll() int64 {
	return PermSendMessages |
		PermManageChannels |
		PermManageRoles |
		PermManageMessages |
		PermAdministrator |
		PermMentionEveryone |
		PermViewChannels |
		PermAttachFiles |
		PermBanMembers |
		PermUseSlashCommands |
		PermManageGuild |
		PermKickMembers
}

var permNames = map[int64]string{
	PermSendMessages:     "send_messages",
	PermManageChannels:   "manage_channels",
	PermManageRoles:      "manage_roles",
	PermManageMessages:   "manage_messages",
	PermAdministrator:    "administrator",
	PermMentionEveryone:  "mention_everyone",
	PermViewChannels:     "view_channels",
	PermAttachFiles:      "attach_files",
	PermBanMembers:       "ban_members",
	PermUseSlashCommands: "use_slash_commands",
	PermManageGuild:      "manage_guild",
	PermKickMembers:      "kick_members",
}

func HasPerm(bitmask, required int64) bool {
	if bitmask&PermAdministrator != 0 {
		return true
	}
	return (bitmask & required) == required
}

func DescribePerms(bitmask int64) []string {
	out := make([]string, 0, len(permNames))
	for bit, name := range permNames {
		if bitmask&bit != 0 {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

type PermissionOverride struct {
	RoleID string `json:"role_id"`
	Allow  int64  `json:"allow"`
	Deny   int64  `json:"deny"`
}

func AllowingOverride(roleID string, bits int64) PermissionOverride {
	return PermissionOverride{RoleID: roleID, Allow: bits, Deny: 0}
}

func DenyingOverride(roleID string, bits int64) PermissionOverride {
	return PermissionOverride{RoleID: roleID, Allow: 0, Deny: bits}
}

func normalizeOverrides(overrides []PermissionOverride) []PermissionOverride {
	if len(overrides) == 0 {
		return nil
	}
	out := make([]PermissionOverride, len(overrides))
	copy(out, overrides)
	return out
}