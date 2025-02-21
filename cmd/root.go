package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "send-pr [target branch]",
	Short: "pr agent",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("請提供 target branch")
			os.Exit(1)
		}
		targetBranch := args[0]

		// 取得當前 branch
		out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
		if err != nil {
			fmt.Println("取得當前 branch 失敗:", err)
			os.Exit(1)
		}
		sourceBranch := strings.TrimSpace(string(out))

		// 取得 git diff 結果
		diffBytes, err := exec.Command("git", "diff", sourceBranch, targetBranch).Output()
		if err != nil {
			fmt.Println("取得 git diff 失敗:", err)
			os.Exit(1)
		}
		diff := string(diffBytes)

		// 呼叫 OpenAI 產生 PR 說明內容
		aiContent, err := generateContent(diff)
		if err != nil {
			fmt.Println("產生 PR 說明內容失敗:", err)
			os.Exit(1)
		}

		// 組合 Markdown 內容
		prTitle := fmt.Sprintf("Merge %s into %s", sourceBranch, targetBranch)
		markdown := fmt.Sprintf("# %s\n\n%s", prTitle, aiContent)

		// 將結果寫入 temporary markdown 檔案
		tmpFile, err := ioutil.TempFile("", "pr_result_*.md")
		if err != nil {
			fmt.Println("建立 temporary file 失敗:", err)
			os.Exit(1)
		}
		defer tmpFile.Close()

		if _, err := tmpFile.Write([]byte(markdown)); err != nil {
			fmt.Println("寫入 temporary file 失敗:", err)
			os.Exit(1)
		}

		fmt.Printf("PR 模擬結果已寫入: %s\n", tmpFile.Name())
	},
}

func generateContent(diff string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("請設定 OPENAI_API_KEY 環境變數")
	}
	client := openai.NewClient(apiKey)
	req := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    "user",
				Content: "請根據下面的 git diff 結果，幫我產出一份 PR 的說明內容：\n" + diff,
			},
		},
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.send-pr.yaml)")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".send-pr")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
