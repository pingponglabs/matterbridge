package config

type ExtraNetworkInfo struct {
	ChannelUsersMember []string          `json:"channel_users_member,omitempty"`
	ActionCommand      string            `json:"action_command,omitempty"`
	ChannelId          string            `json:"channel_id,omitempty"`
	ChannelName        string            `json:"channel_name,omitempty"`
	ChannelType        string            `json:"channel_type,omitempty"`
	TargetPlatform     string            `json:"target_platform,omitempty"`
	UsersMemberId      map[string]string `json:"users_member_id,omitempty"`
	Mentions           map[string]string `json:"mentions,omitempty"`
}
