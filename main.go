package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"

	"golang.org/x/net/context"
	"gopkg.in/alecthomas/kingpin.v2"

	"google-drive-downloader/token"
)

// 動作確認用コマンド
// $ go run ./main.go -f test.wav -t 1ZwI27D9QsYLUdTbU1MgFnpYjH8eipufy
// $ go run ./main.go -f screenshot.png -t 1ZwI27D9QsYLUdTbU1MgFnpYjH8eipufy

// コマンドライン引数
var fileName = kingpin.Flag("file-name", "").Short('f').Required().String()
var outputDir = kingpin.Flag("output-dir", "").Short('o').Default("./").String()
var targetDir = kingpin.Flag("target-dir", "").Short('t').Required().String()

// Google Drive APIアクセス用オブジェクトのシングルトンの保存場所
var service *drive.Service

// 親ディレクトリの情報を保持するように拡張した構造体
type fileInfo struct {
	Parent string
	File   *drive.File
}

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

	// Google Derive APIにアクセスするため使用するサービスを生成します
	service = getService(config)

	// 指定したフォルダ配下から該当ファイルを検索
	files := getFileList(*targetDir, *fileName)

	// 該当ファイルが複数存在する場合
	if len(files) > 1 {
		log.Printf("%sと同名のファイルが%d個存在します。\n", *fileName, len(files))
		for _, f := range files {
			log.Printf("https://drive.google.com/drive/folders/%s\n", f.Parent)
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

	res, err := downloadFile(files[0].File.Id)
	if err != nil {
		log.Fatalf("downloadFile : %v", err)
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

// 指定されたフォルダIDを起点に指定されたファイル名に一致するものを再起的に検索します
func getFileList(id string, name string) (files []fileInfo) {
	q := fmt.Sprintf("'%s' in parents", id)
	list, err := service.Files.List().Q(q).Do()
	if err != nil {
		log.Fatalf("getFileList: %v", err)
	}
	for _, f := range list.Files {
		if f != nil {
			if f.Name == name {
				files = append(files, fileInfo{Parent: id, File: f})
			}
			if f.MimeType == "application/vnd.google-apps.folder" {
				files = append(files, getFileList(f.Id, name)...)
			}
		}
	}

	// 同一フォルダに同名ファイルが存在する可能性があるため重複する情報を削除する
	// フォルダIDをkey、重複判定をvalueとしてrangeで回した時
	// 重複判定が初期値のfalseであればフラグを立てユニークな情報のみスライスに詰める
	m := make(map[string]bool)
	var uniq []fileInfo
	for _, f := range files {
		if m[f.Parent] == false {
			m[f.Parent] = true
			uniq = append(uniq, f)
		}
	}
	return uniq
}

func downloadFile(id string) (res *http.Response, err error) {
	ctx := context.Background()
	for counter := 0; counter < 40; counter++ {
		res, err = service.Files.Get(id).Context(ctx).Download()

		if err != nil {
			log.Printf("\nError file=%s get file retry%d/40\n", id, counter)
			continue
		}
		break
	}
	return res, err
}

func getService(config *oauth2.Config) *drive.Service {
	tokenSource := config.TokenSource(oauth2.NoContext, token.GetToken(config))
	httpClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	s, err := drive.New(httpClient)
	if err != nil {
		log.Fatal(err)
	}
	return s
}

// https://developers.google.com/drive/api/v3/mime-types
func isGoogleDriveMineType(s string) bool {
	return strings.Contains(s, "application/vnd.google-apps")
}
