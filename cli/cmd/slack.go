package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	log_utils "github.com/reviewpad/go-lib/logrus"
	oai "github.com/reviewpad/openai"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(slackCmd)
	slackCmd.Flags().StringVarP(&model, MODEL, "m", "openai-gpt-4", "OpenAI model")
	slackCmd.Flags().StringVarP(&logLevel, LOG_LEVEL, "l", "debug", "Log level")
}

type SlackMessage struct {
	Text         string `json:"text"`
	ResponseType string `json:"response_type"`
}

const systemPrompt string = `
You are an AI Slack bot that lives in Slack servers.

Your mission is to engage in chat conversations with users.

Keep an ethical and professional tone at all times.

You will receive a message from a user and your response should be a message that is appropriate for the conversation.

The next message will be the user message.
`

func chatOnSlack() error {
	logLevel, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return err
	}

	log := log_utils.NewLogger(logLevel)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Error parsing form", http.StatusInternalServerError)
			return
		}

		responseURL := r.FormValue("response_url")
		if responseURL == "" {
			http.Error(w, "No response URL provided", http.StatusBadRequest)
			return
		}

		prompt := r.FormValue("text")
		user := r.FormValue("user_name")
		log.Infof("Received POST request: %v", r.FormValue("text"))

		if err != nil {
			http.Error(w, "Error unmarshalling request body", http.StatusInternalServerError)
			return
		}

		ctx := context.Background()
		client, err := oai.NewOpenAIClient(model)
		if err != nil {
			log.Errorf("failed to create client: %v", err)
			return
		}

		log.Infof("Running with model %s", model)

		reply, err := client.Prompt(
			ctx,
			[]openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		)
		if err != nil {
			return
		}

		sendMessageToSlack(log, responseURL, user, prompt, reply)
	})

	log.Info("Server started on localhost:3001")
	log.Fatal(http.ListenAndServe(":3001", nil))

	return nil
}

func sendMessageToSlack(log *logrus.Entry, responseURL, user, text, reply string) {
	response := user + " prompt: " + text + "\nreply: " + reply

	slackMessage := SlackMessage{
		Text:         response,
		ResponseType: "in_channel",
	}

	log.Infof("Sending message to: %v", responseURL)

	bytesRepresentation, err := json.Marshal(slackMessage)
	if err != nil {
		log.Fatalln(err)
	}

	resp, err := http.Post(responseURL, "application/json", bytes.NewBuffer(bytesRepresentation))
	if err != nil {
		log.Fatalln(err)
	}

	var result map[string]interface{}

	json.NewDecoder(resp.Body).Decode(&result)

	log.Info(result)
}

var slackCmd = &cobra.Command{
	Use: "slack",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 0 {
			return cmd.Help()
		}

		return chatOnSlack()
	},
}
