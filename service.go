package main

import (
	"gitlab-asset-cleaner/utils"
	"log"
	"time"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type Project struct {
	ProjectId          int
	ProjectName        string
	ProjectAccessLevel int
	ProjectLink        string
	PackageId          int
	PackageName        string
	PackageLink        string
	TotalPackageFiles  int
	CreatedAt          *time.Time
}

func Search(client *gitlab.Client) []Project {
	// 연결을 위한 client 초기화
	log.Printf("서비스 시작")
	log.Println("Client is connected")

	// 첫번째 파이프라인 정의 packages 조회 후 다음 파이프라인 채널로..
	resultC := utils.New(func(inC chan interface{}) {
		inC <- nil
		close(inC)
	}, client).Pipe(getProjectexecutor).Pipe(getPackagesExecutor).Merge()

	var resultList []Project
	for result := range resultC {
		if project, ok := result.(Project); ok {
			log.Printf("result <- %v", result)
			resultList = append(resultList, project)
		}
	}

	log.Printf("서비스 완료")

	return resultList
}

func getProjectexecutor(input interface{}, client *gitlab.Client) (interface{}, error) {

	var results []Project

	_, resp, _ := client.Projects.ListProjects(&gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 1,
			Page:    1,
		},
		MinAccessLevel: gitlab.Ptr(gitlab.AccessLevelValue(40)),
	})
	projectCount := resp.TotalItems

	for i := range (projectCount / 100) + 1 {
		result, _, _ := client.Projects.ListProjects(&gitlab.ListProjectsOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: 100,
				Page:    i,
			},
			// Membership:     gitlab.Ptr(true),
			MinAccessLevel: gitlab.Ptr(gitlab.AccessLevelValue(40)),
		})

		for _, p := range result {
			accessLevel := func(project gitlab.Project) int {
				accessLevel := 0
				if project.Permissions == nil {
					return accessLevel
				}

				if project.Permissions.ProjectAccess != nil {
					accessLevel = max(accessLevel, int(project.Permissions.ProjectAccess.AccessLevel))
				}

				if project.Permissions.GroupAccess != nil {
					accessLevel = max(accessLevel, int(project.Permissions.GroupAccess.AccessLevel))
				}

				return accessLevel

			}(*p)

			results = append(results, Project{
				ProjectId:          p.ID,
				ProjectName:        p.Name,
				ProjectAccessLevel: accessLevel,
				ProjectLink:        p.WebURL,
			})
		}
	}

	return results, nil
}

func getPackagesExecutor(input interface{}, client *gitlab.Client) (interface{}, error) {
	project := input.(Project)
	log.Printf("Project: %v", project)
	var resultList []Project

	_, resp, _ := client.Packages.ListProjectPackages(project.ProjectId, &gitlab.ListProjectPackagesOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 1,
		},
	})
	totalPackageCount := resp.TotalItems

	for i := range (totalPackageCount / 100) + 1 {
		packages, _, _ := client.Packages.ListProjectPackages(project.ProjectId, &gitlab.ListProjectPackagesOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: 100,
				Page:    i,
			},
		})

		for _, p := range packages {
			_, resp, _ := client.Packages.ListPackageFiles(project.ProjectId, p.ID, &gitlab.ListPackageFilesOptions{
				PerPage: 1,
			})

			resultList = append(resultList, Project{
				ProjectId:          project.ProjectId,
				ProjectName:        project.ProjectName,
				ProjectAccessLevel: project.ProjectAccessLevel,
				ProjectLink:        project.ProjectLink,
				PackageId:          p.ID,
				PackageName:        p.Name + ": " + p.Version,
				PackageLink:        p.Links.WebPath,
				TotalPackageFiles:  resp.TotalItems,
			})
		}
	}

	log.Printf("resultList: %v", resultList)
	return resultList, nil
}

// func DeletePackageFiles(token string, baseUrl string, projectId string, packageId string) string {
// 	var result string

// 	if client == nil {
// 		client, err = gitlab.NewClient(token, gitlab.WithBaseURL(baseUrl))
// 	}

// 	pkg, _ := strconv.Atoi(packageId)
// 	_, resp, _ := client.Packages.ListPackageFiles(projectId, pkg, &gitlab.ListPackageFilesOptions{
// 		PerPage: 1,
// 	})

// 	totalItemCount := resp.TotalItems
// 	var filesToDelete []*gitlab.PackageFile

// 	if totalItemCount < 20 {
// 		result = "삭제 대상 없음"
// 	} else {
// 		for i := 1; i <= (totalItemCount/100)+1; i++ {
// 			listPackageFiles, _, _ := client.Packages.ListPackageFiles(projectId, pkg, &gitlab.ListPackageFilesOptions{
// 				PerPage: 100,
// 				Page:    i,
// 			})

// 			filesToDelete = append(filesToDelete, listPackageFiles...)
// 		}

// 		sort.Slice(filesToDelete, func(i, j int) bool {
// 			return filesToDelete[i].CreatedAt.After(*filesToDelete[j].CreatedAt)
// 		})

// 		log.Printf("삭제 대상 수: %v", len(filesToDelete)-20)
// 		inC := make(chan int, len(filesToDelete))

// 		for _, packageFile := range filesToDelete[20:] {

// 			inC <- packageFile.ID

// 		}
// 		close(inC)

// 		workerCount := 20
// 		var wg sync.WaitGroup

// 		log.Println("병렬 동작 시작")
// 		for i := 0; i < workerCount; i++ {
// 			wg.Add(1)
// 			go func() {
// 				defer wg.Done()
// 				for v := range inC {
// 					log.Printf("Project: %v, Package: %v", projectId, pkg)
// 					_, err := client.Packages.DeletePackageFile(projectId, pkg, v)
// 					if err != nil {
// 						log.Fatalf("Error deleteing file %d: %v", v, err)
// 					}
// 				}
// 			}()
// 		}
// 		wg.Wait()
// 		result = "성공"
// 	}

// 	return result

// }
