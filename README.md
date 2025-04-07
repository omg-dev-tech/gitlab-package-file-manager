## Gitlab Package File Manager

[![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Echo](https://img.shields.io/badge/Echo-00B5E2?style=for-the-badge&logo=go&logoColor=white)](https://echo.labstack.com/)
[![jQuery](https://img.shields.io/badge/jQuery-0769AD?style=for-the-badge&logo=jquery&logoColor=white)](https://jquery.com/)
[![Bootstrap](https://img.shields.io/badge/Bootstrap-7952B3?style=for-the-badge&logo=bootstrap&logoColor=white)](https://getbootstrap.com/)
[![Bootstrap Table](https://img.shields.io/badge/Bootstrap_Table-7952B3?style=for-the-badge&logo=bootstrap&logoColor=white)](https://bootstrap-table.com/)

### For What?
GitLab does not provide a feature to manage multiple file assets within the same version of a package registry. Additionally, there is no dashboard that shows the overall package registry usage for specific users (tokens) and their permissions. Therefore, this program was created to provide such a dashboard interface on the web.

### Usage

#### Running Locally
```bash
go run --port=8080
```

#### Build Instructions

##### For Windows
```bash
# Build for Windows
GOOS=windows GOARCH=amd64 go build -o gitlab-package-file-manager.exe
```

##### For Linux
```bash
# Build for Linux
GOOS=linux GOARCH=amd64
go build -o gitlab-package-file-manager
```

##### For macOS
```bash
# Build for macOS
GOOS=darwin GOARCH=amd64
go build -o gitlab-package-file-manager
```

#### Running the Built Binary

##### Windows
```bash
./gitlab-package-file-manager.exe --port=8080
```

##### Linux/macOS
```bash
./gitlab-package-file-manager --port=8080
```

Note: Make sure you have Go installed on your system before building the application.