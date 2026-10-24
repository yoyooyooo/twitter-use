package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yoyooyooo/twitter-use/internal/client"
	"github.com/yoyooyooo/twitter-use/internal/errs"
	"github.com/yoyooyooo/twitter-use/internal/model"
	"github.com/yoyooyooo/twitter-use/internal/output"
	"github.com/yoyooyooo/twitter-use/internal/search"
	"github.com/yoyooyooo/twitter-use/internal/timeutil"
)

func addRead(root *cobra.Command, a *app) {
	root.AddCommand(
		homeCmd(a), bookmarksCmd(a), userCmd(a), postsCmd(a), searchCmd(a), likesCmd(a),
		tweetCmd(a), articleCmd(a), followersCmd(a), followingCmd(a), whoamiCmd(a),
	)
}

type pageFlags struct {
	max                  int
	since, until, cursor string
	full                 bool
	output               string
}

func addPageFlags(cmd *cobra.Command, f *pageFlags, timed bool) {
	cmd.Flags().IntVarP(&f.max, "max", "n", 0, "Max items")
	cmd.Flags().StringVar(&f.cursor, "cursor", "", "Resume cursor")
	if timed {
		cmd.Flags().StringVar(&f.since, "since", "", "Keep items at or after this time (YYYY-MM-DD or RFC3339)")
		cmd.Flags().StringVar(&f.until, "until", "", "Keep items at or before this time (YYYY-MM-DD or RFC3339)")
	}
	cmd.Flags().BoolVar(&f.full, "full-text", false, "Do not truncate text in the table")
	cmd.Flags().StringVarP(&f.output, "output", "o", "", "Write JSON to this file")
}

func (a *app) window(since, until string) (time.Time, time.Time, error) {
	return timeutil.ParseWindow(since, until)
}

func (a *app) applyWindow(api *client.Client, since, until string) error {
	lower, upper, err := a.window(since, until)
	if err != nil {
		return errs.Input(err.Error())
	}
	api.SetWindow(lower, upper)
	return nil
}

func homeCmd(a *app) *cobra.Command {
	var f pageFlags
	var feedType string
	cmd := &cobra.Command{Use: "home", Short: "Home timeline for the current identity", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.tweets(cmd, false, "home", f.max, f.full, f.output, func(api *client.Client, count int) ([]model.Tweet, error) {
			if err := a.applyWindow(api, f.since, f.until); err != nil {
				return nil, err
			}
			if feedType == "following" {
				return api.FollowingFeed(count, false, f.cursor)
			}
			return api.Home(count, false, f.cursor)
		})
	}}
	addPageFlags(cmd, &f, true)
	cmd.Flags().StringVarP(&feedType, "type", "t", "for-you", "for-you or following")
	return cmd
}

func bookmarksCmd(a *app) *cobra.Command {
	var f pageFlags
	cmd := &cobra.Command{Use: "bookmarks", Short: "Bookmarks for the current identity", RunE: func(cmd *cobra.Command, _ []string) error {
		return a.tweets(cmd, false, "bookmarks", f.max, f.full, f.output, func(api *client.Client, count int) ([]model.Tweet, error) {
			if err := a.applyWindow(api, f.since, f.until); err != nil {
				return nil, err
			}
			return api.Bookmarks(count, f.cursor)
		})
	}}
	addPageFlags(cmd, &f, true)
	folders := &cobra.Command{Use: "folders [id]", Short: "List bookmark folders, or read one folder", RunE: func(cmd *cobra.Command, args []string) error {
		api, cfg, err := a.open(cmd, false)
		if err != nil {
			return a.fail(err)
		}
		if len(args) == 0 {
			list, err := api.BookmarkFolders()
			if err != nil {
				return a.fail(err)
			}
			if f.output != "" {
				_ = writeJSON(f.output, output.Folders(list))
			}
			if err := a.emit(output.Success(output.Folders(list))); err == nil {
				return nil
			}
			for _, folder := range list {
				fmt.Fprintf(a.out, "%s\t%s\n", folder.ID, folder.Name)
			}
			return nil
		}
		count, err := fetchCount(f.max, cfg)
		if err != nil {
			return a.fail(err)
		}
		if err := a.applyWindow(api, f.since, f.until); err != nil {
			return a.fail(err)
		}
		tweets, err := api.BookmarkFolder(args[0], count, f.cursor)
		if err != nil && len(tweets) == 0 {
			return a.fail(err)
		}
		return a.finishTweets(api, "folder "+args[0], tweets, f.full, f.output)
	}}
	addPageFlags(folders, &f, true)
	cmd.AddCommand(folders)
	return cmd
}

func userCmd(a *app) *cobra.Command {
	return &cobra.Command{Use: "user [screen_name]", Short: "Public profile. Omit the name to use the current identity", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		screen := screenArg(args)
		api, _, err := a.open(cmd, screen != "")
		if err != nil {
			return a.fail(err)
		}
		var profile model.UserProfile
		if screen == "" {
			profile, err = api.Me()
		} else {
			profile, err = api.User(screen)
		}
		if err != nil {
			return a.fail(err)
		}
		if err := a.emit(output.Success(output.User(profile))); err == nil {
			return nil
		}
		fmt.Fprintf(a.out, "@%s\t%s\n%s\nfollowers %d  following %d  tweets %d\n", profile.ScreenName, profile.Name, profile.Bio, profile.FollowersCount, profile.FollowingCount, profile.TweetsCount)
		return nil
	}}
}

func postsCmd(a *app) *cobra.Command {
	var f pageFlags
	cmd := &cobra.Command{Use: "posts [screen_name]", Short: "Posts by a user. Omit the name to use the current identity", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		screen := screenArg(args)
		return a.tweets(cmd, screen != "", postsLabel(screen), f.max, f.full, f.output, func(api *client.Client, count int) ([]model.Tweet, error) {
			if err := a.applyWindow(api, f.since, f.until); err != nil {
				return nil, err
			}
			id, err := a.userID(api, screen)
			if err != nil {
				return nil, err
			}
			return api.UserTweets(id, count, f.cursor)
		})
	}}
	addPageFlags(cmd, &f, true)
	return cmd
}

func searchCmd(a *app) *cobra.Command {
	var product, from, to, lang, since, until, outputFile, cursor string
	var has, exclude []string
	var minLikes, minRetweets, max int
	var full bool
	cmd := &cobra.Command{Use: "search [query]", Short: "Search posts", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		text := ""
		if len(args) == 1 {
			text = args[0]
		}
		var likes, rts *int
		if cmd.Flags().Changed("min-likes") {
			likes = &minLikes
		}
		if cmd.Flags().Changed("min-retweets") {
			rts = &minRetweets
		}
		query, err := search.Build(search.Query{Text: text, From: from, To: to, Lang: lang, Since: since, Until: until, Has: has, Exclude: exclude, MinLikes: likes, MinRetweets: rts})
		if err != nil {
			return a.fail(errs.Input(err.Error()))
		}
		if query == "" {
			return a.fail(errs.Input("Provide a query or at least one advanced filter"))
		}
		return a.tweets(cmd, true, query, max, full, outputFile, func(api *client.Client, count int) ([]model.Tweet, error) {
			return api.Search(query, count, product, cursor)
		})
	}}
	cmd.Flags().StringVarP(&product, "product", "t", "Top", "Top, Latest, Photos, or Videos")
	cmd.Flags().StringVar(&from, "from", "", "Only this author")
	cmd.Flags().StringVar(&to, "to", "", "Only replies to this author")
	cmd.Flags().StringVar(&lang, "lang", "", "Language code")
	cmd.Flags().StringVar(&since, "since", "", "YYYY-MM-DD, sent to X as since:")
	cmd.Flags().StringVar(&until, "until", "", "YYYY-MM-DD, sent to X as until:")
	cmd.Flags().StringSliceVar(&has, "has", nil, "links, images, videos, or media")
	cmd.Flags().StringSliceVar(&exclude, "exclude", nil, "retweets, replies, or links")
	cmd.Flags().IntVar(&minLikes, "min-likes", 0, "Minimum likes")
	cmd.Flags().IntVar(&minRetweets, "min-retweets", 0, "Minimum retweets")
	cmd.Flags().IntVarP(&max, "max", "n", 0, "Max tweets")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Resume cursor")
	cmd.Flags().BoolVar(&full, "full-text", false, "Do not truncate text in the table")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write JSON")
	return cmd
}

func likesCmd(a *app) *cobra.Command {
	var f pageFlags
	cmd := &cobra.Command{Use: "likes [screen_name]", Short: "Likes. Omit the name to use the current identity; a name uses the pool", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		screen := screenArg(args)
		return a.tweets(cmd, screen != "", "likes", f.max, f.full, f.output, func(api *client.Client, count int) ([]model.Tweet, error) {
			if err := a.applyWindow(api, f.since, f.until); err != nil {
				return nil, err
			}
			id, err := a.userID(api, screen)
			if err != nil {
				return nil, err
			}
			return api.Likes(id, count, f.cursor)
		})
	}}
	addPageFlags(cmd, &f, true)
	return cmd
}

func tweetCmd(a *app) *cobra.Command {
	var f pageFlags
	var ranking string
	cmd := &cobra.Command{Use: "tweet [id]", Short: "A post and its replies", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := tweetID(args[0])
		if err != nil {
			return a.fail(err)
		}
		api, cfg, err := a.open(cmd, true)
		if err != nil {
			return a.fail(err)
		}
		count, err := fetchCount(f.max, cfg)
		if err != nil {
			return a.fail(err)
		}
		if err := a.applyWindow(api, f.since, f.until); err != nil {
			return a.fail(err)
		}
		tweets, err := api.TweetDetail(id, count, ranking, f.cursor)
		if err != nil && len(tweets) == 0 {
			return a.fail(err)
		}
		return a.thread(api, tweets, id, ranking, f.full)
	}}
	addPageFlags(cmd, &f, true)
	cmd.Flags().StringVar(&ranking, "ranking", "Relevance", "Relevance or Recency")
	return cmd
}

func articleCmd(a *app) *cobra.Command {
	var asMarkdown bool
	var outputFile string
	cmd := &cobra.Command{Use: "article [id]", Short: "Fetch a long-form article", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if a.compact {
			return a.fail(errs.Input("`twitter article` does not support --compact"))
		}
		id, err := tweetID(args[0])
		if err != nil {
			return a.fail(err)
		}
		api, _, err := a.open(cmd, true)
		if err != nil {
			return a.fail(err)
		}
		tweet, err := api.Article(id)
		if err != nil {
			return a.fail(err)
		}
		title, body := "", ""
		if tweet.ArticleTitle != nil {
			title = *tweet.ArticleTitle
		}
		if tweet.ArticleText != nil {
			body = *tweet.ArticleText
		}
		markdown := title
		if body != "" {
			if markdown != "" {
				markdown += "\n\n"
			}
			markdown += body
		}
		if outputFile != "" {
			if err := os.WriteFile(outputFile, []byte(markdown), 0o600); err != nil {
				return a.fail(err)
			}
		}
		if asMarkdown {
			fmt.Fprintln(a.out, markdown)
			return nil
		}
		if err := a.emit(output.Success(output.Tweet(tweet))); err == nil {
			return nil
		}
		fmt.Fprintln(a.out, markdown)
		return nil
	}}
	cmd.Flags().BoolVarP(&asMarkdown, "markdown", "m", false, "Print Markdown")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write Markdown")
	return cmd
}

func followersCmd(a *app) *cobra.Command { return usersCmd(a, "followers", true) }
func followingCmd(a *app) *cobra.Command { return usersCmd(a, "following", false) }

func usersCmd(a *app, name string, followers bool) *cobra.Command {
	var max int
	var cursor string
	cmd := &cobra.Command{Use: name + " [screen_name]", Short: "Accounts related to a user. Omit the name to use the current identity", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		screen := screenArg(args)
		api, cfg, err := a.open(cmd, screen != "")
		if err != nil {
			return a.fail(err)
		}
		count, err := fetchCount(max, cfg)
		if err != nil {
			return a.fail(err)
		}
		id, err := a.userID(api, screen)
		if err != nil {
			return a.fail(err)
		}
		var users []model.UserProfile
		if followers {
			users, err = api.Followers(id, count, cursor)
		} else {
			users, err = api.Following(id, count, cursor)
		}
		if err != nil && len(users) == 0 {
			return a.fail(err)
		}
		payload := output.Success(output.Users(users))
		payload["pagination"] = pagination(api, false)
		if err := a.emit(payload); err == nil {
			return nil
		}
		for _, user := range users {
			fmt.Fprintf(a.out, "@%s\t%s\n", user.ScreenName, user.Name)
		}
		return nil
	}}
	cmd.Flags().IntVarP(&max, "max", "n", 0, "Max accounts")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Resume cursor")
	return cmd
}

func (a *app) userID(api *client.Client, screen string) (string, error) {
	if screen == "" {
		profile, err := api.Me()
		if err != nil {
			return "", err
		}
		if profile.ID == "" {
			return "", errs.API(0, "current identity has no user id")
		}
		return profile.ID, nil
	}
	return api.ResolveUserID(screen)
}

func (a *app) thread(api *client.Client, tweets []model.Tweet, id, ranking string, full bool) error {
	notice := pagination(api, false)
	notice["ranking"] = ranking
	for _, tweet := range tweets {
		if tweet.ID == id && tweet.Metrics.Replies > 0 {
			notice["platformReplies"] = tweet.Metrics.Replies
		}
	}
	payload := output.Success(output.Tweets(tweets))
	payload["pagination"] = notice
	if err := a.emit(payload); err == nil {
		return nil
	}
	printTweets(a.out, "thread "+id, tweets, full)
	return nil
}

func (a *app) finishTweets(api *client.Client, label string, tweets []model.Tweet, full bool, outputFile string) error {
	if outputFile != "" {
		_ = writeJSON(outputFile, output.Tweets(tweets))
	}
	payload := output.Success(output.Tweets(tweets))
	payload["pagination"] = pagination(api, false)
	if err := a.emit(payload); err == nil {
		return nil
	}
	printTweets(a.out, label, tweets, full)
	return nil
}

func screenArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(args[0]), "@")
}

func postsLabel(screen string) string {
	if screen == "" {
		return "posts"
	}
	return "@" + screen
}
