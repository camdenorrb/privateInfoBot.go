package data

type RSSType string

const (
	Reddit           RSSType = "Reddit"
	Github           RSSType = "Github"
	TitleAndLink     RSSType = "TitleAndLink"
	KernelOrgUpdates RSSType = "KernelOrgUpdates"
)

type RSSFeed struct {
	ChannelName  string   `json:"channelName"`
	FeedURL      string   `json:"feedURL"`
	Color        *string  `json:"color,omitempty"`
	Title        *string  `json:"title,omitempty"`
	Description  *string  `json:"description,omitempty"`
	ThumbnailURL *string  `json:"thumbnailURL,omitempty"`
	Type         *RSSType `json:"type,omitempty"`
	Author       *string  `json:"author,omitempty"`
	FileName     *string  `json:"fileName,omitempty"`
}
