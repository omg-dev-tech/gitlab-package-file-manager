package main

import (
	"gitlab-asset-cleaner/utils"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"fmt"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type Project struct {
	ProjectId          int
	ProjectName        string
	ProjectAccessLevel int
	ProjectLink        string
	PackageId          int
	PackageName        string
	PackageVersion     string
	PackageLink        string
	TotalPackageFiles  int
	CreatedAt          *time.Time
}

type PackageFile struct {
	ProjectId     int
	PackageId     int
	PackageFileId int
	CreatedAt     *time.Time
}

func Search(client *gitlab.Client, projectName string, packageName string, fromFileCount string, toFileCount string) []Project {
	log.Printf("조회 서비스 시작")

	resultC := utils.New(func(inC chan interface{}) {
		inC <- nil
		close(inC)
	}, client).Pipe(func(input interface{}, client *gitlab.Client, workerId int, options ...any) (interface{}, error) {
		log.Printf("[worker-%v] first start", workerId)
		projectName := options[0].(string)
		log.Printf("검색 옵션: %v", projectName)
		var results []Project

		_, resp, _ := client.Projects.ListProjects(&gitlab.ListProjectsOptions{
			Search: &projectName,
			ListOptions: gitlab.ListOptions{
				PerPage: 1,
				Page:    1,
			},
			MinAccessLevel: gitlab.Ptr(gitlab.AccessLevelValue(40)),
		})
		projectCount := resp.TotalItems

		for i := range (projectCount / 100) + 1 {
			result, _, _ := client.Projects.ListProjects(&gitlab.ListProjectsOptions{
				Search: &projectName,
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
		log.Printf("[worker-%v] first done | %v", workerId, results)
		return results, nil
	}, projectName).Pipe(func(input interface{}, client *gitlab.Client, workerId int, options ...any) (interface{}, error) {
		project := input.(Project)
		packageName := options[0].(string)

		var resultList []Project
		log.Printf("[worker-%v] second start", workerId)
		_, resp, _ := client.Packages.ListProjectPackages(project.ProjectId, &gitlab.ListProjectPackagesOptions{
			PackageName: &packageName,
			ListOptions: gitlab.ListOptions{
				PerPage: 1,
			},
		})
		totalPackageCount := resp.TotalItems

		from, _ := strconv.Atoi(fromFileCount)
		to, err := strconv.Atoi(toFileCount)
		if err != nil || to == 0 {
			to = 9999
		}

		for i := range (totalPackageCount / 100) + 1 {
			packages, _, _ := client.Packages.ListProjectPackages(project.ProjectId, &gitlab.ListProjectPackagesOptions{
				PackageName: &packageName,
				ListOptions: gitlab.ListOptions{
					PerPage: 100,
					Page:    i,
				},
			})

			for _, p := range packages {
				_, resp, _ := client.Packages.ListPackageFiles(project.ProjectId, p.ID, &gitlab.ListPackageFilesOptions{
					PerPage: 1,
				})
				if resp.TotalItems < from || resp.TotalItems > to {
					continue
				}

				resultList = append(resultList, Project{
					ProjectId:          project.ProjectId,
					ProjectName:        project.ProjectName,
					ProjectAccessLevel: project.ProjectAccessLevel,
					ProjectLink:        project.ProjectLink,
					PackageId:          p.ID,
					PackageName:        p.Name,
					PackageVersion:     p.Version,
					PackageLink:        strings.Join([]string{strings.ReplaceAll(client.BaseURL().String(), "/gitlab/api/v4/", ""), p.Links.WebPath}, ""),
					TotalPackageFiles:  resp.TotalItems,
				})
			}
		}
		log.Printf("[worker-%v] second done | %v", workerId, resultList)
		return resultList, nil
	}, packageName).Merge()

	var resultList []Project
	for result := range resultC {
		if project, ok := result.(Project); ok {
			// log.Printf("result <- %v", result)
			resultList = append(resultList, project)
		}
	}

	log.Printf("조회 서비스 완료")

	return resultList
}

func Clean(client *gitlab.Client, cleanupPackageFiles interface{}) []string {

	log.Printf("정리 서비스 시작")

	resultC := utils.New(func(inC chan interface{}) {
		for _, packageFile := range cleanupPackageFiles.([]PackageFile) {
			log.Printf("대상 파일: %v", packageFile)
			inC <- packageFile
		}
		close(inC)
	}, client).Pipe(func(input interface{}, client *gitlab.Client, workerId int, options ...any) (interface{}, error) {
		log.Printf("[worker-%v] start", workerId)
		var filesToDelete []PackageFile
		packageFile := input.(PackageFile)
		_, resp, _ := client.Packages.ListPackageFiles(packageFile.ProjectId, packageFile.PackageId, &gitlab.ListPackageFilesOptions{
			PerPage: 1,
		})
		totalItemCount := resp.TotalItems
		if totalItemCount < 20 {
			return nil, nil
		} else {
			for i := 1; i <= (totalItemCount/100)+1; i++ {
				listPackageFiles, _, _ := client.Packages.ListPackageFiles(packageFile.ProjectId, packageFile.PackageId, &gitlab.ListPackageFilesOptions{
					PerPage: 100,
					Page:    i,
				})
				for _, _packageFile := range listPackageFiles {

					filesToDelete = append(filesToDelete, PackageFile{
						ProjectId:     packageFile.ProjectId,
						PackageId:     packageFile.PackageId,
						PackageFileId: _packageFile.ID,
						CreatedAt:     _packageFile.CreatedAt,
					})
				}
			}
			sort.Slice(filesToDelete, func(i, j int) bool {
				return filesToDelete[i].CreatedAt.After(*filesToDelete[j].CreatedAt)
			})
			log.Printf("filesToDelete: %v", filesToDelete)
			log.Printf("[worker-%v] done", workerId)
			return filesToDelete[20:], nil
		}
	}).Pipe(func(input interface{}, client *gitlab.Client, workerId int, a ...any) (interface{}, error) {
		log.Printf("[worker-%v] start", workerId)
		packageFile := input.(PackageFile)

		response, err := client.Packages.DeletePackageFile(packageFile.ProjectId, packageFile.PackageId, packageFile.PackageFileId)
		log.Printf("[worker-%v] done", workerId)
		if err != nil {
			return fmt.Sprint("%d, %d, %d Error: %v", packageFile.ProjectId, packageFile.PackageId, packageFile.PackageFileId, err), err
		} else {
			return response, nil
		}
	}).Merge()

	var resultList []string
	for result := range resultC {
		if response, ok := result.(string); ok {
			log.Printf("result <- %v", result)
			resultList = append(resultList, response)
		}
	}

	log.Printf("정리 서비스 완료")

	return resultList
}

func Statics(token *string, client *gitlab.Client) interface{} {

	return nil
}
