package main

import (
	"gitlab-asset-cleaner/utils"
	"log"

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
		projects, err := getProjects()
		if err != nil {
			log.Fatalf("Failed to get projects: %v", err)
		}

		for project := range projects {
			inC <- project
		}

	}).Pipe(getPackagesExecutor).Merge()

	resultList := <-resultC

	return resultList.([]Project)
}

func getProjects() ([]Project, error) {
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
	return results, nil
}

func getPackagesExecutor(input interface{}) (interface{}, error) {

	project := input.(Project)

	var packageList []Package

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
			totalPackageFileCount := resp.TotalItems

			packageList = append(packageList, Package{
				PackageId:         p.ID,
				PackageName:       p.Name,
				TotalPackageFiles: totalPackageFileCount,
			})

		}
	}

	project.Packages = packageList

	return project, nil
}
