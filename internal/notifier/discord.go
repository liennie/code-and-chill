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

func (n *DiscordNotifier) notify(ctx context.Context, event, title, name, snippet, link string) error {
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

	_, err = n.cli.ChannelMessageSendComplex(
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

	return nil
}
