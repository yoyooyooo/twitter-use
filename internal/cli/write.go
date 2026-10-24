package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yoyooyooo/twitter-use/internal/output"
)

func addWrite(root *cobra.Command, a *app) {
	root.AddCommand(likeCmd(a), unlikeCmd(a), retweetCmd(a), unretweetCmd(a), bookmarkCmd(a), unbookmarkCmd(a), followCmd(a), unfollowCmd(a))
}

func whoamiCmd(a *app) *cobra.Command {
	return &cobra.Command{Use: "whoami", Short: "Show the current identity", RunE: func(cmd *cobra.Command, _ []string) error {
		api, _, err := a.open(cmd, false)
		if err != nil {
			return a.fail(err)
		}
		profile, err := api.Me()
		if err != nil {
			return a.fail(err)
		}
		if err := a.emit(output.Success(map[string]any{"user": output.User(profile)})); err == nil {
			return nil
		}
		fmt.Fprintf(a.out, "@%s\t%s\n", profile.ScreenName, profile.Name)
		return nil
	}}
}

type writeAPI interface {
	Like(string) error
	Unlike(string) error
	Retweet(string) error
	Unretweet(string) error
	Bookmark(string) error
	Unbookmark(string) error
	Follow(string) error
	Unfollow(string) error
	ResolveUserID(string) (string, error)
}

func (a *app) mutate(cmd *cobra.Command, action, id string, fn func(writeAPI) error) error {
	api, _, err := a.open(cmd, false)
	if err != nil {
		return a.fail(err)
	}
	if err := fn(api); err != nil {
		return a.fail(err)
	}
	payload := output.Success(map[string]any{"action": action, "id": id})
	if err := a.emit(payload); err == nil {
		return nil
	}
	fmt.Fprintf(a.out, "%s\t%s\n", action, id)
	return nil
}

func idAction(a *app, use, action string, fn func(writeAPI, string) error) *cobra.Command {
	return &cobra.Command{Use: use + " [id]", Short: action + " as the current identity", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id, err := tweetID(args[0])
		if err != nil {
			return a.fail(err)
		}
		return a.mutate(cmd, use, id, func(api writeAPI) error { return fn(api, id) })
	}}
}

func likeCmd(a *app) *cobra.Command {
	return idAction(a, "like", "Like a post", func(api writeAPI, id string) error { return api.Like(id) })
}
func unlikeCmd(a *app) *cobra.Command {
	return idAction(a, "unlike", "Remove a like", func(api writeAPI, id string) error { return api.Unlike(id) })
}
func retweetCmd(a *app) *cobra.Command {
	return idAction(a, "retweet", "Repost", func(api writeAPI, id string) error { return api.Retweet(id) })
}
func unretweetCmd(a *app) *cobra.Command {
	return idAction(a, "unretweet", "Undo a repost", func(api writeAPI, id string) error { return api.Unretweet(id) })
}
func bookmarkCmd(a *app) *cobra.Command {
	return idAction(a, "bookmark", "Bookmark a post", func(api writeAPI, id string) error { return api.Bookmark(id) })
}
func unbookmarkCmd(a *app) *cobra.Command {
	return idAction(a, "unbookmark", "Remove a bookmark", func(api writeAPI, id string) error { return api.Unbookmark(id) })
}

func followCmd(a *app) *cobra.Command {
	return userAction(a, "follow", func(api writeAPI, id string) error { return api.Follow(id) })
}
func unfollowCmd(a *app) *cobra.Command {
	return userAction(a, "unfollow", func(api writeAPI, id string) error { return api.Unfollow(id) })
}

func userAction(a *app, action string, fn func(writeAPI, string) error) *cobra.Command {
	return &cobra.Command{Use: action + " [screen_name]", Short: action + " as the current identity", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		api, _, err := a.open(cmd, false)
		if err != nil {
			return a.fail(err)
		}
		id, err := api.ResolveUserID(strings.TrimPrefix(args[0], "@"))
		if err != nil {
			return a.fail(err)
		}
		if err := fn(api, id); err != nil {
			return a.fail(err)
		}
		payload := output.Success(map[string]any{"action": action, "screenName": strings.TrimPrefix(args[0], "@"), "id": id})
		if err := a.emit(payload); err == nil {
			return nil
		}
		fmt.Fprintf(a.out, "%s\t@%s\n", action, strings.TrimPrefix(args[0], "@"))
		return nil
	}}
}
