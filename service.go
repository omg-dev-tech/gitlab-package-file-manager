package main

import (
	"gitlab-package-file-manager/utils"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"fmt"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type Project struct {
	Description         string
	ProjectId           int
	ProjectName         string
	ProjectAccessLevel  int
	ProjectLink         string
	PackageRegistrySize int
	CreatedAt           *time.Time
	Owner               string
	RepositorySize      int
}

type Package struct {
	PackageId        int
	PackageFileCount int
	PackageFileSize  int
	PackageLink      string
	PackageName      string
	ProjectId        int
	Version          string
}

func GetPackages(client *gitlab.Client, projectId int) []Package {
	log.Printf("project id: %v", projectId)
	resultC := utils.New(func(inC chan interface{}) {
		_, resp, _ := client.Packages.ListProjectPackages(projectId, &gitlab.ListProjectPackagesOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: 1,
				Page:    1,
			},
		})

		if resp.TotalItems != 0 {
			for i := range (resp.TotalItems-1)/100 + 1 {
				packages, _, _ := client.Packages.ListProjectPackages(projectId, &gitlab.ListProjectPackagesOptions{
					ListOptions: gitlab.ListOptions{
						PerPage: 100,
						Page:    i + 1,
					},
				})
				log.Printf("new done| %v", packages)
				for _, _package := range packages {
					inC <- _package
				}
			}
		}

		close(inC)
	}, client).Pipe(func(input interface{}, client *gitlab.Client, workerId int, options ...any) (interface{}, error) {

		_packageInput := input.(*gitlab.Package)
		var _packageOutput Package

		_packageOutput.ProjectId = projectId
		_packageOutput.PackageId = _packageInput.ID
		_packageOutput.PackageName = _packageInput.Name
		_packageOutput.PackageLink = strings.Join([]string{strings.ReplaceAll(client.BaseURL().String(), "/gitlab/api/v4/", ""), _packageInput.Links.WebPath}, "")
		_packageOutput.Version = _packageInput.Version

		_, resp, _ := client.Packages.ListPackageFiles(projectId, _packageInput.ID, &gitlab.ListPackageFilesOptions{
			PerPage: 1,
			Page:    1,
		})

		_packageOutput.PackageFileCount = resp.TotalItems

		if resp.TotalItems == 0 {
			return _packageOutput, nil
		}

		for i := range (resp.TotalItems-1)/100 + 1 {
			packageFiles, _, _ := client.Packages.ListPackageFiles(projectId, _packageInput.ID, &gitlab.ListPackageFilesOptions{
				PerPage: 100,
				Page:    i + 1,
			})

			for _, packageFile := range packageFiles {
				_packageOutput.PackageFileSize += packageFile.Size
			}
		}

		_packageOutput.PackageFileSize = _packageOutput.PackageFileSize / 1024 / 1024
		return _packageOutput, nil

	}, "").Merge()

	var resultList []Package
	for result := range resultC {
		if _package, ok := result.(Package); ok {
			resultList = append(resultList, _package)
		}
	}

	return resultList
}

func Clean(client *gitlab.Client, cleanupPackageFiles interface{}) []string {

	resultC := utils.New(func(inC chan interface{}) {
		for _, packageFile := range cleanupPackageFiles.([]Package) {
			if packageFile.PackageId == 0 {
				packageList, _, _ := client.Packages.ListProjectPackages(packageFile.ProjectId, nil, nil)

				for _, info := range packageList {
					inC <- Package{
						ProjectId: packageFile.ProjectId,
						PackageId: info.ID,
					}
				}
			} else {
				inC <- packageFile
			}
		}
		close(inC)
	}, client).Pipe(func(input interface{}, client *gitlab.Client, workerId int, options ...any) (interface{}, error) {
		var filesToDelete []map[string]interface{}
		packageFile := input.(Package)
		_, resp, _ := client.Packages.ListPackageFiles(packageFile.ProjectId, packageFile.PackageId, &gitlab.ListPackageFilesOptions{
			PerPage: 1,
		})
		totalItemCount := resp.TotalItems
		if totalItemCount < 20 {
			return nil, nil
		} else {
			for i := range (totalItemCount-1)/100 + 1 {
				listPackageFiles, _, _ := client.Packages.ListPackageFiles(packageFile.ProjectId, packageFile.PackageId, &gitlab.ListPackageFilesOptions{
					PerPage: 100,
					Page:    i + 1,
				})
				for _, _packageFile := range listPackageFiles {

					filesToDelete = append(filesToDelete, map[string]interface{}{
						"ProjectId":     packageFile.ProjectId,
						"PackageId":     packageFile.PackageId,
						"PackageFileId": _packageFile.ID,
						"CreatedAt":     _packageFile.CreatedAt,
					})
				}
			}
			sort.Slice(filesToDelete, func(i, j int) bool {
				return filesToDelete[i]["CreatedAt"].(*time.Time).After(*filesToDelete[j]["CreatedAt"].(*time.Time))
			})
			return filesToDelete[20:], nil
		}
	}).Pipe(func(input interface{}, client *gitlab.Client, workerId int, a ...any) (interface{}, error) {
		packageFile := input.(map[string]interface{})

		response, err := client.Packages.DeletePackageFile(packageFile["ProjectId"], packageFile["PackageId"].(int), packageFile["PackageFileId"].(int))
		if err != nil {
			return fmt.Sprint("Project ID: %d, Package ID: %d, Package File ID: %d - Delete Failed: %v",
				packageFile["ProjectId"], packageFile["PackageId"], packageFile["PackageFileId"], err), err
		} else {
			return response, nil
		}
	}).Merge()

	var resultList []string
	for result := range resultC {
		if response, ok := result.(string); ok {
			resultList = append(resultList, response)
		}
	}

	return resultList
}

func GetProjects(client *gitlab.Client, offset int, limit int, projectName string, fromSize string, toSize string) []Project {
	isStatistics := true
	_, resp, _ := client.Projects.ListProjects(&gitlab.ListProjectsOptions{
		Search: &projectName,
		ListOptions: gitlab.ListOptions{
			PerPage: 1,
			Page:    1,
		},
		MinAccessLevel: gitlab.Ptr(gitlab.AccessLevelValue(40)),
	})
	totalCount := resp.TotalItems
	var results []Project

	if totalCount == 0 {
		return results
	}

	for i := range (totalCount-1)/100 + 1 {
		result, _, _ := client.Projects.ListProjects(&gitlab.ListProjectsOptions{
			Search: &projectName,
			ListOptions: gitlab.ListOptions{
				PerPage: 100,
				Page:    i + 1,
			},
			Statistics:     &isStatistics,
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

			from, _ := strconv.Atoi(fromSize)
			to, err := strconv.Atoi(toSize)
			if err != nil || to == 0 {
				to = 999999
			}
			packageRegistrySize := int(p.Statistics.PackagesSize) / 1024 / 1024

			if from <= packageRegistrySize && packageRegistrySize <= to {
				ownerName := ""
				if p.Owner != nil {
					ownerName = p.Owner.Username
				}

				results = append(results, Project{

					ProjectId:           p.ID,
					ProjectName:         p.Name,
					ProjectAccessLevel:  accessLevel,
					ProjectLink:         p.WebURL,
					PackageRegistrySize: packageRegistrySize,
					Description:         p.Description,
					CreatedAt:           p.CreatedAt,
					Owner:               ownerName,
					RepositorySize:      int(p.Statistics.RepositorySize) / 1024 / 1024,
				})
			}
		}
	}

	return results
}
