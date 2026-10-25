package model

import "testing"

func TestSplitThreadOrdersAncestorsAndKeepsOtherPostsAsReplies(t *testing.T) {
	tweets := []Tweet{
		{ID: "root", Text: "root"},
		{ID: "child", InReplyToStatusID: "root"},
		{ID: "focal", InReplyToStatusID: "child"},
		{ID: "reply-1", InReplyToStatusID: "focal"},
		{ID: "reply-2"},
	}
	focal, ancestors, replies := SplitThread(tweets, "focal")
	if focal == nil || focal.ID != "focal" {
		t.Fatalf("focal=%v", focal)
	}
	if len(ancestors) != 2 || ancestors[0].ID != "root" || ancestors[1].ID != "child" {
		t.Fatalf("ancestors=%v", ancestors)
	}
	if len(replies) != 2 || replies[0].ID != "reply-1" || replies[1].ID != "reply-2" {
		t.Fatalf("replies=%v", replies)
	}
}

func TestSplitThreadWithoutFocalKeepsEveryPostAsReply(t *testing.T) {
	focal, ancestors, replies := SplitThread([]Tweet{{ID: "a"}}, "missing")
	if focal != nil || len(ancestors) != 0 || len(replies) != 1 || replies[0].ID != "a" {
		t.Fatalf("focal=%v ancestors=%v replies=%v", focal, ancestors, replies)
	}
}
