package module

import (
	"github.com/stretchr/testify/assert"
	"path"
	"privateInfoBot/data"
	"testing"
)

func TestRSSUpdateModule_filePath(test *testing.T) {

	tests := []struct {
		testName         string
		url              string
		expectedFileName string
	}{
		{
			testName:         "kernelOrg",
			url:              "https://www.kernel.org/feeds/kdist.xml",
			expectedFileName: "kernel.org_feeds_kdist.xml.json",
		},
		{
			testName:         "redditLongevity",
			url:              "https://www.reddit.com/r/longevity/.rss",
			expectedFileName: "reddit.com_r_longevity_.rss.json",
		},
	}

	for _, testData := range tests {
		test.Run(testData.testName, func(test *testing.T) {

			module := &RSSUpdateModule{
				rssFeed: data.RSSFeed{FeedURL: testData.url},
			}

			expectedFilePath := path.Join("Modules", "RSS", testData.expectedFileName)
			assert.Equal(test, expectedFilePath, module.filePath())
		})
	}
}
