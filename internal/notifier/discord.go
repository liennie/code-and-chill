package notifier

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type DiscordNotifier struct {
	cli      *discordgo.Session
	channels map[string]ChannelConfig
}

func newDiscordNotifier(config *DiscordConfig) *DiscordNotifier {
	if config == nil {
		return nil
	}

	if config.Token == "" {
		panic("discord notifier: token is required")
	}

	token, err := os.ReadFile(config.Token)
	if err != nil {
		panic(fmt.Errorf("discord notifier: read discord token: %w", err))
	}

	cli, err := discordgo.New("Bot " + strings.TrimSpace(string(token)))
	if err != nil {
		panic(fmt.Errorf("discord notifier: client: %w", err))
	}

	return &DiscordNotifier{
		cli:      cli,
		channels: config.Channels,
	}
}

func (n *DiscordNotifier) notify(ctx context.Context, event, title, name, snippet, link string, cleanup bool) error {
	chanID, ok := n.channels[event]
	if !ok || chanID.Notifications == "" || chanID.Forum == "" {
		return fmt.Errorf("missing channels for event %q", event)
	}

	ch, err := n.cli.ForumThreadStartComplex(
		chanID.Forum,
		&discordgo.ThreadStart{
			Name: name,
		},
		&discordgo.MessageSend{
			Content: fmt.Sprintf("You can discuss the puzzle **%s** here.", name),
			Embeds: []*discordgo.MessageEmbed{{
				URL:         link,
				Type:        discordgo.EmbedTypeLink,
				Title:       title,
				Description: snippet,
				Color:       0x5865f2,
			}},
		},
		discordgo.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("discord notifier: thread start: %w", err)
	}

	if cleanup {
		_, err = n.cli.ChannelDelete(ch.ID, discordgo.WithContext(ctx))
		if err != nil {
			return fmt.Errorf("discord notifier: cleanup thread: %w", err)
		}
	}

	msg, err := n.cli.ChannelMessageSendComplex(
		chanID.Notifications,
		&discordgo.MessageSend{
			Content: fmt.Sprintf("# New puzzle %q is now available!\n\nVisit %s to start solving.\n\nJoin the discussion in thread %s.\n\nGood luck, have fun, and happy puzzling! @everyone", name, link, ch.Mention()),
			Embeds: []*discordgo.MessageEmbed{{
				URL:         link,
				Type:        discordgo.EmbedTypeLink,
				Title:       title,
				Description: snippet,
				Color:       0x5865f2,
			}},
		},
		discordgo.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("discord notifier: notification: %w", err)
	}

	if cleanup {
		err = n.cli.ChannelMessageDelete(chanID.Notifications, msg.ID, discordgo.WithContext(ctx))
		if err != nil {
			return fmt.Errorf("discord notifier: cleanup notification: %w", err)
		}
	}

	return nil
}

// SetupResult reports the IDs of the forum threads created by a setup call.
// Fields may be empty if the corresponding thread was not created.
type SetupResult struct {
	NotificationsThreadID string `json:"notificationsThreadId,omitempty"`
	GeneralThreadID       string `json:"generalThreadId,omitempty"`
}

func (n *DiscordNotifier) setup(ctx context.Context, eventPath, eventName string) (SetupResult, error) {
	var result SetupResult

	chanID, ok := n.channels[eventPath]
	if !ok || chanID.Forum == "" {
		return result, fmt.Errorf("missing forum for event %q", eventPath)
	}

	notifTh, err := n.cli.ForumThreadStartComplex(
		chanID.Forum,
		&discordgo.ThreadStart{
			Name: "Notifications",
		},
		&discordgo.MessageSend{
			Content: fmt.Sprintf("Follow this thread to receive notifications about new puzzles for the **%s** event.", eventName),
		},
		discordgo.WithContext(ctx),
	)
	if err != nil {
		return result, fmt.Errorf("discord notifier: setup notifications thread: %w", err)
	}
	result.NotificationsThreadID = notifTh.ID

	locked, archived := true, false
	_, err = n.cli.ChannelEditComplex(
		notifTh.ID,
		&discordgo.ChannelEdit{
			Locked:   &locked,
			Archived: &archived,
		},
		discordgo.WithContext(ctx),
	)
	if err != nil {
		return result, fmt.Errorf("discord notifier: setup lock notifications thread: %w", err)
	}

	pinned := discordgo.ChannelFlagPinned
	_, err = n.cli.ChannelEditComplex(
		notifTh.ID,
		&discordgo.ChannelEdit{
			Flags: &pinned,
		},
		discordgo.WithContext(ctx),
	)
	if err != nil {
		return result, fmt.Errorf("discord notifier: setup pin notifications thread: %w", err)
	}

	genTh, err := n.cli.ForumThreadStartComplex(
		chanID.Forum,
		&discordgo.ThreadStart{
			Name: "General discussion",
		},
		&discordgo.MessageSend{
			Content: fmt.Sprintf("Welcome! Use this thread for general discussion about the **%s** event.", eventName),
		},
		discordgo.WithContext(ctx),
	)
	if err != nil {
		return result, fmt.Errorf("discord notifier: setup general thread: %w", err)
	}
	result.GeneralThreadID = genTh.ID

	return result, nil
}
