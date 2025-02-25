package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"log/slog"

	"github.com/blackhorseya/send-pr/internal/prompt"
	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	defaultModel       = openai.O3Mini
	defaultTemperature = 1.0
	defaultTopP        = 1.0
)

var cfgFile string
var verbose bool

var rootCmd = &cobra.Command{
	Use:   "send-pr [target branch]",
	Short: "pr agent",
	PreRun: func(cmd *cobra.Command, args []string) {
		// 設定 logger 等級：只有帶 -v 才顯示 debug log
		if verbose {
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			})))
		} else {
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			})))
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			slog.Error("Target branch not provided")
			fmt.Println("請提供 target branch")
			os.Exit(1)
		}
		targetBranch := args[0]
		slog.Debug("Target branch", "targetBranch", targetBranch)

		// 取得當前 branch
		slog.Debug("Getting current branch")
		out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
		if err != nil {
			slog.Error("Failed to get current branch", "error", err)
			fmt.Println("取得當前 branch 失敗:", err)
			os.Exit(1)
		}
		sourceBranch := strings.TrimSpace(string(out))
		slog.Debug("Current branch", "sourceBranch", sourceBranch)

		// 取得 git diff 結果
		slog.Debug("Getting git diff", "sourceBranch", sourceBranch, "targetBranch", targetBranch)
		diffBytes, err := exec.Command("git", "diff", targetBranch, sourceBranch).Output()
		if err != nil {
			slog.Error("Failed to get git diff", "error", err)
			fmt.Println("取得 git diff 失敗:", err)
			os.Exit(1)
		}
		diff := string(diffBytes)
		slog.Debug("Git diff result", "diff", diff)

		// 呼叫 OpenAI 產生 PR 說明內容
		slog.Debug("Generating PR content with OpenAI")
		aiContent, err := generateContent(diff)
		if err != nil {
			slog.Error("Failed to generate PR content", "error", err)
			fmt.Println("產生 PR 說明內容失敗:", err)
			os.Exit(1)
		}
		slog.Debug("Generated AI content", "aiContent", aiContent)

		// 組合 Markdown 內容
		prTitle := fmt.Sprintf("Merge %s into %s", sourceBranch, targetBranch)
		markdown := fmt.Sprintf("# %s\n\n%s", prTitle, aiContent)
		slog.Debug("Assembled markdown", "markdown", markdown)

		// 將結果寫入 temporary markdown 檔案
		slog.Debug("Writing markdown to temporary file")
		tmpFile, err := os.CreateTemp("", "pr_result_*.md")
		if err != nil {
			slog.Error("Failed to create temporary file", "error", err)
			fmt.Println("建立 temporary file 失敗:", err)
			os.Exit(1)
		}
		defer tmpFile.Close()

		if _, err := tmpFile.Write([]byte(markdown)); err != nil {
			slog.Error("Failed to write to temporary file", "error", err)
			fmt.Println("寫入 temporary file 失敗:", err)
			os.Exit(1)
		}

		fmt.Printf("PR 模擬結果已寫入: %s\n", tmpFile.Name())
	},
}

func generateContent(diff string) (string, error) {
	content, err := prompt.GetPromptString(prompt.SummarizePRDiffTemplate, map[string]interface{}{
		"file_diffs": diff,
	})
	if err != nil {
		return "", fmt.Errorf("取得 prompt 失敗: %w", err)
	}
	slog.Debug("Prompt content", "content", content)

	client, err := initOpenAIClient()
	if err != nil {
		return "", err
	}

	req := openai.ChatCompletionRequest{
		Model: defaultModel,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "You are a helpful AI assistant that helps write a PR description based on the diff of the source and target branches.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: content,
			},
		},
		Temperature: defaultTemperature,
		TopP:        defaultTopP,
	}
	resp, err := client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("OpenAI 回傳空結果")
	}
	return resp.Choices[0].Message.Content, nil
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/send-pr/.send-pr.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		configDir := fmt.Sprintf("%s/.config/send-pr", home)
		viper.AddConfigPath(configDir)
		viper.SetConfigName(".send-pr")
		viper.SetConfigType("yaml")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func initOpenAIClient() (*openai.Client, error) {
	apiKey := viper.GetString("openai.api_key")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("請設定 openai.api_key 或 OPENAI_API_KEY 環境變數")
	}

	baseURL := viper.GetString("openai.base_url")
	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		slog.Debug("使用自訂的 OpenAI BaseURL", "baseURL", baseURL)
		config.BaseURL = baseURL
	}
	return openai.NewClientWithConfig(config), nil
}
