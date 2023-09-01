package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	oai "github.com/reviewpad/openai"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	log_utils "github.com/trailblazing-reviewpad/chatty/logrus"
)

const (
	DISCORD_TOKEN = "DISCORD_TOKEN"
)

func init() {
	rootCmd.AddCommand(discordCmd)
	discordCmd.Flags().StringVarP(&model, MODEL, "m", "openai-gpt-4", "OpenAI model")
	discordCmd.Flags().StringVarP(&logLevel, LOG_LEVEL, "l", "debug", "Log level")
}

const discordSystemPrompt string = `
You are an AI Discord chatbot that lives in Discord servers.
Your mission is to engage in chat conversations with users.

Keep an ethical and professional tone at all times.

You will receive a message from a user and your response should be a message that is appropriate for the conversation.

The next message will be the user message.
`

func chatOnDiscord() error {
	ctx := context.Background()

	logLevel, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return err
	}

	log := log_utils.NewLogger(logLevel)

	client, err := oai.NewOpenAIClient(model)
	if err != nil {
		log.Errorf("failed to create client: %v", err)
		return err
	}

	discordToken, val := os.LookupEnv(DISCORD_TOKEN)
	if !val {
		log.Errorf("failed to get discord token")
		return fmt.Errorf("failed to get discord token")
	}

	log.Infof("Initialzing Chatty Discord with model %s", model)

	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return nil
	}

	// Register callback for MessageCreate events.
	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}

		// log the message
		log.Infof("%s: %s\n%v\n", m.Author.Username, m.Content, m)

		reply, err := client.Prompt(
			ctx,
			[]openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: discordSystemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: m.Content,
				},
			},
		)

		if err != nil {
			log.Errorf("failed to get reply: %v", err)
			return
		}

		// log the reply
		log.Infof("Reply: %s\n", reply)

		_, err = s.ChannelMessageSend(m.ChannelID, reply)
		if err != nil {
			log.Errorln("error sending DM message: ", err)
		}
	})

	// Set the intents.
	dg.Identify.Intents = discordgo.IntentsAllWithoutPrivileged

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		log.Errorln("error opening connection,", err)
		return nil
	}

	// Wait here until CTRL-C or other term signal is received.
	log.Infof("Running Chatty Discord. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Close down the Discord session.
	dg.Close()

	return nil
}

var discordCmd = &cobra.Command{
	Use: "discord",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 {
			return cmd.Help()
		}

		return chatOnDiscord()
	},
}
