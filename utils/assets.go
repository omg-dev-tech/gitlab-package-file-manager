package utils

import (
	"log"
	"sort"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// var listPackageFilesOptions = gitlab.ListPackageFilesOptions{
// 	ListOptions: listOptions,
// }

type FileInfo struct {
	ID        int
	CreatedAt time.Time
	PID       int
	PVID      int
}

func GetAllAsset(git *gitlab.Client, pid int, pvid int, remain int) []FileInfo {
	totalItem := getTotalItem(git, pid, pvid)
	perPage := 100
	var fileList []FileInfo

	for i := 1; i <= (totalItem/perPage)+1; i++ {
		var opt = gitlab.ListPackageFilesOptions{
			PerPage: 100,
			Page:    i,
		}
		files, _, _ := git.Packages.ListPackageFiles(pid, pvid, &opt, nil)

		for _, f := range files {

			fileList = append(fileList, FileInfo{
				ID:        f.ID,
				CreatedAt: *f.CreatedAt,
				PID:       pid,
				PVID:      pvid,
			})
		}

	}

	log.Printf("Total files fetched: %d", len(fileList))

	// fileList 를 sort 함
	sort.Slice(fileList, func(i, j int) bool {
		return fileList[i].CreatedAt.After(fileList[j].CreatedAt)
	})

	var filesToDelete []FileInfo
	if len(fileList) > remain {
		filesToDelete = fileList[remain:]
	} else {
		log.Printf("삭제 대상 파일이 없습니다. (전체 파일 수가 %v개 이하)", remain)
		return []FileInfo{}
	}
	log.Printf("Files to delete: %d", len(filesToDelete))

	return filesToDelete

}

func getTotalItem(git *gitlab.Client, pid int, pvid int) int {
	var searchOpt = gitlab.ListPackageFilesOptions{
		PerPage: 1,
	}
	_, resp, _ := git.Packages.ListPackageFiles(pid, pvid, &searchOpt, nil)

	return resp.TotalItems
}
