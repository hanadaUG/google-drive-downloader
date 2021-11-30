package main

import (
	"fmt"
	"google-drive-downloader/helper"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"

	"gopkg.in/alecthomas/kingpin.v2"
)

// 動作確認用コマンド
// $ go run ./main.go -f test.wav -t 1ZwI27D9QsYLUdTbU1MgFnpYjH8eipufy
// $ go run ./main.go -f screenshot.png -t 1ZwI27D9QsYLUdTbU1MgFnpYjH8eipufy

// コマンドライン引数
var fileName = kingpin.Flag("file-name", "").Short('f').Required().String()
var outputDir = kingpin.Flag("output-dir", "").Short('o').Default("./").String()
var targetDir = kingpin.Flag("target-dir", "").Short('t').Required().String()

func main() {
	kingpin.Parse()
	log.Printf("%sのダウンロードを開始します。\n", *fileName)

	// Google Derive APIにアクセスするため
	// OAuth 2.0 クライアントIDの認証情報を読み込む
	b, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// jsonからコンフィグに変換
	config, err := google.ConfigFromJSON(b, drive.DriveReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	// GoogleDriveへのアクセスを補助する構造体を初期化
	g, err := helper.NewGoogleDrive(config)
	if err != nil {
		log.Fatalf("Unable to init google drive client: %v", err)
	}

	// 指定したフォルダ配下から該当ファイルを検索
	files := g.GetFileList(*targetDir, *fileName)

	// 該当ファイルが複数存在する場合
	if len(files) > 1 {
		log.Printf("%sと同名のファイルが%d個存在します。\n", *fileName, len(files))
		for _, f := range files {
			log.Printf("https://helper.google.com/helper/folders/%s\n", f.Parent)
		}
		log.Println("いずれかのファイルをリネームして重複状態を解決してください。")
		os.Exit(1)
	}

	// 見つからない場合
	if len(files) == 0 {
		log.Fatalf("%sが見つかりませんでした。", *fileName)
	}

	// 一つだけ見つかった場合
	if isGoogleDriveMineType(files[0].File.MimeType) {
		log.Fatalln("GoogleDrive固有のファイル種類だったためダウンロードを中断しました。")
	}

	res, err := g.Download(files[0].File.Id)
	if err != nil {
		log.Fatalf("Download: %v", err)
	}

	file, err := os.Create(fmt.Sprintf("%s/%s", *outputDir, *fileName))
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	_, err = io.Copy(file, res.Body)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("ダウンロードが完了しました。")
}

// https://developers.google.com/drive/api/v3/mime-types
func isGoogleDriveMineType(s string) bool {
	return strings.Contains(s, "application/vnd.google-apps")
}
