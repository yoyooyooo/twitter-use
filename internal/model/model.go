package model

// Author is the account that posted a tweet.
type Author struct {
	ID              string
	Name            string
	ScreenName      string
	ProfileImageURL string
	Verified        bool
}

// Metrics are public engagement counts.
type Metrics struct {
	Likes     int
	Retweets  int
	Replies   int
	Quotes    int
	Views     int
	Bookmarks int
}

// TweetMedia is one attached photo, video, or gif.
type TweetMedia struct {
	Type   string
	URL    string
	Width  *int
	Height *int
}

// Tweet is one status, including a quoted status when present.
type Tweet struct {
	ID               string
	Text             string
	Author           Author
	Metrics          Metrics
	CreatedAt        string
	Media            []TweetMedia
	URLs             []string
	IsRetweet        bool
	Lang             string
	RetweetedBy      string
	QuotedTweet      *Tweet
	Score            *float64
	ArticleTitle     *string
	ArticleText      *string
	IsSubscriberOnly bool
	IsPromoted       bool
}

// BookmarkFolder is a named bookmark collection.
type BookmarkFolder struct {
	ID   string
	Name string
}

// UserProfile is a public account profile.
type UserProfile struct {
	ID              string
	Name            string
	ScreenName      string
	Bio             string
	Location        string
	URL             string
	FollowersCount  int
	FollowingCount  int
	TweetsCount     int
	LikesCount      int
	Verified        bool
	ProfileImageURL string
	CreatedAt       string
}

// FetchStatus explains why a paged read stopped.
type FetchStatus struct {
	Requested  int
	Returned   int
	Stop       string
	NextCursor string
}
