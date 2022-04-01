package templates

import _ "embed"

var (
	//go:embed resource/hello.txt
	Hello string
	//go:embed resource/initializationError.txt
	InitializationError string
	//go:embed resource/userNotStarted.txt
	UserNotStarted string
	//go:embed resource/unexpectedError.txt
	UnexpectedError string
	//go:embed resource/channelList.txt
	ChannelList string
	//go:embed resource/noChannels.txt
	NoChannels string
	//go:embed resource/emptyAdd.txt
	EmptyAdd string
	//go:embed resource/addSuccess.txt
	AddSuccess string
	//go:embed resource/customUrlUnsupported.txt
	CustomUrlUnsupported string
	//go:embed resource/removeSuccess.txt
	RemoveSuccess string
	//go:embed resource/upcoming.txt
	Upcoming string
	//go:embed resource/live.txt
	Live string
)
