package main

import (
	"gitlab-asset-cleaner/utils"
	"log"
	"sync"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

var client *gitlab.Client
var err error

func Search(token string, baseUrl string) []Project {
	// 연결을 위한 client 초기화
	client, err = gitlab.NewClient(token, gitlab.WithBaseURL(baseUrl))
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	log.Println("Client is connected")

	// 첫번째 파이프라인 정의 packages 조회 후 다음 파이프라인 채널로..
	resultC := utils.New(func(inC chan interface{}) {
		defer close(inC)
		// 패키지 정보를 channel 에 넣어줌
		projects := getProjects()

		for project := range projects {
			inC <- project
		}

	}).Pipe()

	// 두번째 파이프라인 정의

	// 세번째 파이프라인 정의

	return resultList
}

func getProjects() []Project {
	if err != nil {
		return nil, err
	}
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
			})
		}
	}
	return results
}

func getPackagesExecutor(input interface{}) (interface{}, error) {

	project := input.(Project)

	packageC := make(chan []interface{})

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
			packageC <- []interface{}{project.ProjectId, p}
		}

	}

	var wg sync.WaitGroup
	numWorkers := 10

	for i := 0; i < numWorkers; i++ {
		wg.Add(i)

		go func() {
			defer wg.Done()
			for info := range packageC {
				projectId := info[0].(int)
				packageInfo := info[1].(gitlab.Package)
				_, resp, _ := client.Packages.ListPackageFiles(projectId, packageInfo.ID, &gitlab.ListPackageFilesOptions{
					PerPage: 1,
				})

				for j := range (resp.TotalItems / 100) + 1 {
					packageFile, _, _ := client.Packages.ListPackageFiles(projectId, packageInfo.ID, &gitlab.ListPackageFilesOptions{
						PerPage: 100,
						Page:    j,
					})

				}
			}
		}()
	}
	// channel 에 넣기

	return _, nil
}

func listProjectPackages(projectId int) {
	var resultList []Registry
	// get one to use one
	_, resp, _ := client.Packages.ListProjectPackages(projectId, &gitlab.ListProjectPackagesOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 1,
		},
	})
	totalItems := resp.TotalItems

	for i := range (totalItems / 100) + 1 {
		packageLists, _, _ := client.Packages.ListProjectPackages(projectId, &gitlab.ListProjectPackagesOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: 100,
				Page:    i,
			},
		})

		for _, onePackage := range packageLists {
			resultList = append(resultList, Registry{
				ID:        onePackage.ID,
				Name:      onePackage.Name,
				TotalItem: onePackage,
			})
		}
	}

}
