package main

import (
	"encoding/json"
	"flag"
	"gitlab-asset-cleaner/utils"
	"log"
	"sync"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type ProjectInfo struct {
	PID  int
	PVID []int
}

func main() {

	token := flag.String("token", "", "API 인증 토큰")
	jsonInput := flag.String("info", "", "JSON 형식의 입력 (예: '{\"key\":\"value\"}')")

	flag.Parse()

	if *token == "" {
		log.Fatalln("토큰 값은 필수입니다.")
		return
	}

	if *jsonInput == "" {
		log.Fatalln("알맞은 프로젝트 정보를 입력하세요.")
		return
	}
	var projectInfoList []ProjectInfo
	if err := json.Unmarshal([]byte(*jsonInput), &projectInfoList); err != nil {
		log.Fatalf("JSON 파싱 중 오류 발생: %v", err)
	}

	log.Println("Asset Delete Start!!")

	baseURL := "https://git.bwg.co.kr/gitlab/api/v4"
	client, err := gitlab.NewClient(*token, gitlab.WithBaseURL(baseURL))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	log.Println("Client is connected")

	var deleteList []utils.FileInfo
	for _, projectInfo := range projectInfoList {

		// calculate total page

		for _, pvid := range projectInfo.PVID {

			// 633, 5779
			deleteList = append(deleteList, utils.GetAllAsset(client, projectInfo.PID, pvid)...)
		}
	}

	// 삭제를 위한 채널에 삭제할 파일 ID를 넣습니다.
	fileChan := make(chan utils.FileInfo, len(deleteList))
	for _, file := range deleteList {
		fileChan <- file
	}
	close(fileChan)

	var deleteWg sync.WaitGroup
	numDeleteWorkers := 10
	for i := 0; i < numDeleteWorkers; i++ {
		deleteWg.Add(1)
		go func(workerID int) {
			defer deleteWg.Done()
			for file := range fileChan {
				// 삭제 API 호출 (예: DeletePackageFile 함수 사용)
				_, err := client.Packages.DeletePackageFile(file.PID, file.PVID, file.ID, nil)
				if err != nil {
					log.Printf("[Worker %d] Error deleting file %d: %v", workerID, file.ID, err)
				} else {
					log.Printf("[Worker %d] Successfully deleted file %d", workerID, file.ID)
				}
			}
		}(i)
	}

	deleteWg.Wait()
	log.Println("Deletion process completed.")

}
