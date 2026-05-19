package banter

const (
	IntentGuilds           int64 = 1 << 0
	IntentGuildMembers     int64 = 1 << 1
	IntentGuildModeration  int64 = 1 << 2
	IntentGuildPresences   int64 = 1 << 3
	IntentGuildMessages    int64 = 1 << 4
	IntentGuildReactions   int64 = 1 << 5
	IntentGuildTyping      int64 = 1 << 6
	IntentGuildVoiceStates int64 = 1 << 7
	IntentDirectMessages   int64 = 1 << 8
	IntentDirectReactions  int64 = 1 << 9
	IntentDirectTyping     int64 = 1 << 10
	IntentMessageContent   int64 = 1 << 11
	IntentBotEvents        int64 = 1 << 12
)

func IntentsDefault() int64 {
	return IntentGuilds |
		IntentGuildMessages |
		IntentGuildReactions |
		IntentGuildMembers |
		IntentBotEvents
}

func IntentsAll() int64 {
	return IntentGuilds |
		IntentGuildMembers |
		IntentGuildModeration |
		IntentGuildPresences |
		IntentGuildMessages |
		IntentGuildReactions |
		IntentGuildTyping |
		IntentGuildVoiceStates |
		IntentDirectMessages |
		IntentDirectReactions |
		IntentDirectTyping |
		IntentMessageContent |
		IntentBotEvents
}

func IntentsNone() int64 { return 0 }