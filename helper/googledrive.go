package helper

import (
	"fmt"
	"log"
	"net/http"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"

	"google-drive-downloader/token"
)

// GoogleDrive GoogleDriveへのアクセスを補助する構造体
type GoogleDrive struct {
	serviceInstance *drive.Service
}

// 親ディレクトリの情報を保持するように拡張した構造体
type fileInfo struct {
	Parent string
	File   *drive.File
}

func NewGoogleDrive(config *oauth2.Config) (*GoogleDrive, error) {
	tokenSource := config.TokenSource(oauth2.NoContext, token.GetToken(config))
	httpClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	serviceInstance, err := drive.New(httpClient)
	return &GoogleDrive{serviceInstance}, err
}

// Download 指定されたファイルIDのファイルをダウンローそする
func (g *GoogleDrive) Download(id string) (res *http.Response, err error) {
	ctx := context.Background()
	for counter := 0; counter < 40; counter++ {
		res, err = g.serviceInstance.Files.Get(id).Context(ctx).Download()
		if err != nil {
			log.Printf("Download Error: id=%s. %d/40 retry\n", id, counter)
			continue
		}
		break
	}
	return res, err
}

// GetFileList 指定されたフォルダIDを起点に指定されたファイル名に一致するものを再起的に検索します
func (g *GoogleDrive) GetFileList(id string, name string) []fileInfo {
	var files []fileInfo
	q := fmt.Sprintf("'%s' in parents", id)
	list, err := g.serviceInstance.Files.List().Q(q).Do()
	if err != nil {
		log.Fatalf("GetFileList: %v", err)
	}
	for _, f := range list.Files {
		if f != nil {
			if f.Name == name {
				files = append(files, fileInfo{Parent: id, File: f})
			}
			if f.MimeType == "application/vnd.google-apps.folder" {
				files = append(files, g.GetFileList(f.Id, name)...)
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
