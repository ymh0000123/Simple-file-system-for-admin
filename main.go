package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"io/ioutil"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v2"
)

type File struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	IsDir    bool   `json:"is_dir"`
	Password string `json:"password"`
}

type Config struct {
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type AdminStats struct {
	UploadCount   int
	DownloadCount int
	LogContent    string
	Files         []File
}

var (
	uploadCount   int
	downloadCount int
	mutex         sync.Mutex
)

func main() {
	// 设置日志文件
	logFile, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal("无法打开日志文件:", err)
	}
	defer logFile.Close()

	// 设置日志格式
	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime)

	// 读取配置文件
	configData, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Fatal("无法读取配置文件:", err)
	}

	// 解析配置值
	var config Config
	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		log.Fatal("无法解析配置文件:", err)
	}

	// 检查uploads目录是否存在，如果不存在则创建
	uploadsDir := "uploads"
	err = os.MkdirAll(uploadsDir, 0755)
	if err != nil {
		log.Fatal("无法创建uploads目录:", err)
	}

	r := gin.Default()

	r.Static("/uploads", "./uploads")

	r.GET("/", func(c *gin.Context) {
		// 首页的 HTML 内容
		log.Printf("[%s] [IP: %s] 访问首页", time.Now().Format(time.RFC3339), c.ClientIP())
		indexHTML := `
			<!DOCTYPE html>
			<html>
			<head>
				<title>文件上传</title>
				<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/twitter-bootstrap/5.0.2/css/bootstrap.min.css">
			</head>
			<body>
				<div class="container">
					<h1>文件上传</h1>
					<form action="/upload" method="post" enctype="multipart/form-data">
						<div class="mb-3">
							<label for="file" class="form-label">选择文件</label>
							<input class="form-control" type="file" name="file" id="file" required>
						</div>
						<button type="submit" class="btn btn-primary">上传</button>
					</form>
					<br>
					<a href="/list" class="btn btn-secondary">文件列表</a><a href="/admin" class="btn btn-primary">管理员界面</a>
				</div>
				<script src="https://cdnjs.cloudflare.com/ajax/libs/twitter-bootstrap/5.0.2/js/bootstrap.bundle.min.js"></script>
			</body>
			</html>
		`

		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, indexHTML)
	})

	r.GET("/list", func(c *gin.Context) {
		// 获取文件列表的逻辑
		files := getFileList("uploads")

		// 文件列表的 HTML 内容
		log.Printf("[%s] [IP: %s] 访问文件列表", time.Now().Format(time.RFC3339), c.ClientIP())
		listHTML := `
			<!DOCTYPE html>
			<html>
			<head>
				<title>文件列表</title>
				<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/twitter-bootstrap/5.0.2/css/bootstrap.min.css">
			</head>
			<body>
				<div class="container">
					<h1>文件列表</h1>
					<table class="table">
						<thead>
							<tr>
								<th>文件名</th>
								<th>操作</th>
							</tr>
						</thead>
						<tbody>
							{{ range $index, $file := .Files }}
								<tr>
									<td>{{ $file.Name }}</td>
									<td>
										<a href="/file/{{ $file.Name }}/direct-link" class="btn btn-primary btn-sm">复制直链</a>
										<a href="/file/{{ $file.Name }}" class="btn btn-secondary btn-sm">查看详情</a>
									</td>
								</tr>
							{{ end }}
						</tbody>
					</table>
				</div>
				<script src="https://cdnjs.cloudflare.com/ajax/libs/twitter-bootstrap/5.0.2/js/bootstrap.bundle.min.js"></script>
			</body>
			</html>
		`

		tmpl, err := template.New("list").Parse(listHTML)
		if err != nil {
			log.Println("无法解析模板:", err)
			c.String(http.StatusInternalServerError, "无法解析模板")
			return
		}

		c.Header("Content-Type", "text/html; charset=utf-8")
		err = tmpl.Execute(c.Writer, gin.H{
			"Files": files,
		})
		if err != nil {
			log.Println("无法渲染模板:", err)
			c.String(http.StatusInternalServerError, "无法渲染模板")
			return
		}
	})

	r.GET("/file/:name", func(c *gin.Context) {
		name := c.Param("name")

		// 根据文件名获取文件信息的逻辑
		fileURL := "/uploads/" + name
		fileInfo := File{
			Name:     name,
			URL:      fileURL,
			IsDir:    false,
			Password: "your_password",
		}

		// 单个文件信息的 HTML 内容
		fileHTML := `
			<!DOCTYPE html>
			<html>
			<head>
				<title>文件详情</title>
				<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/twitter-bootstrap/5.0.2/css/bootstrap.min.css">
			</head>
			<body>
				<div class="container">
					<h1>文件详情</h1>
					<p>文件名：{{ .Name }}</p>
					<p>文件URL：<a href="{{ .URL }}">{{ .URL }}</a></p>
				</div>
				<script src="https://cdnjs.cloudflare.com/ajax/libs/twitter-bootstrap/5.0.2/js/bootstrap.bundle.min.js"></script>
			</body>
			</html>
		`

		tmpl, err := template.New("file").Parse(fileHTML)
		if err != nil {
			log.Println("无法解析模板:", err)
			c.String(http.StatusInternalServerError, "无法解析模板")
			return
		}

		c.Header("Content-Type", "text/html; charset=utf-8")
		err = tmpl.Execute(c.Writer, fileInfo)
		if err != nil {
			log.Println("无法渲染模板:", err)
			c.String(http.StatusInternalServerError, "无法渲染模板")
			return
		}
	})

	r.GET("/file/:name/direct-link", func(c *gin.Context) {
		name := c.Param("name")

		// 根据文件名获取文件信息的逻辑
		fileURL := c.Request.Host + "/uploads/" + name

		c.String(http.StatusOK, fileURL)
	})

	// 管理员面板路由组
	admin := r.Group("/admin", gin.BasicAuth(gin.Accounts{config.Username: config.Password}))

	// 管理员面板首页
	admin.GET("/", func(c *gin.Context) {
		// 获取今天上传的文件数量
		uploadCount := getTodayUploadCount()
		// 获取今天下载的文件数量
		downloadCount := getTodayDownloadCount()

		// 读取日志文件内容
		logContent, err := ioutil.ReadFile("app.log")
		if err != nil {
			log.Println("无法读取日志文件:", err)
			c.String(http.StatusInternalServerError, "无法读取日志文件")
			return
		}

		// 获取文件列表
		files := getFileList("uploads")

		// 管理员面板的 HTML 内容
		adminHTML := `
			<!DOCTYPE html>
			<html>
			<head>
				<title>管理员面板</title>
				<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/twitter-bootstrap/5.0.2/css/bootstrap.min.css">
			</head>
			<body>
				<div class="container">
					<h1>管理员面板</h1>
					<h2>今日统计</h2>
					<p>上传文件数量：{{ .UploadCount }}</p>
					<p>下载文件数量：{{ .DownloadCount }}</p>
					<h2>日志</h2>
					<pre>{{ .LogContent }}</pre>
					<h2>文件列表</h2>
					<table class="table">
						<thead>
							<tr>
								<th>文件名</th>
								<th>操作</th>
							</tr>
						</thead>
						<tbody>
							{{ range $index, $file := .Files }}
								<tr>
									<td>{{ $file.Name }}</td>
									<td>
										<a href="/admin/file/{{ $file.Name }}/delete" class="btn btn-danger btn-sm">删除</a>
									</td>
								</tr>
							{{ end }}
						</tbody>
					</table>
					<a href="/admin/logout" class="btn btn-secondary">退出登录</a>
				</div>
				<script src="https://cdnjs.cloudflare.com/ajax/libs/twitter-bootstrap/5.0.2/js/bootstrap.bundle.min.js"></script>
			</body>
			</html>
		`

		tmpl, err := template.New("admin").Parse(adminHTML)
		if err != nil {
			log.Println("无法解析模板:", err)
			c.String(http.StatusInternalServerError, "无法解析模板")
			return
		}

		c.Header("Content-Type", "text/html; charset=utf-8")
		err = tmpl.Execute(c.Writer, AdminStats{
			UploadCount:   uploadCount,
			DownloadCount: downloadCount,
			LogContent:    string(logContent),
			Files:         files,
		})
		if err != nil {
			log.Println("无法渲染模板:", err)
			c.String(http.StatusInternalServerError, "无法渲染模板")
			return
		}
	})

	admin.GET("/logout", func(c *gin.Context) {
		c.Header("WWW-Authenticate", `Basic realm="Authorization Required"`)
		c.AbortWithStatus(http.StatusUnauthorized)
	})

	admin.GET("/file/:name/delete", func(c *gin.Context) {
		name := c.Param("name")

		// 删除文件的逻辑
		err := deleteFile(name)
		if err != nil {
			log.Println("无法删除文件:", err)
			c.String(http.StatusInternalServerError, "无法删除文件")
			return
		}
		log.Printf("[%s] [IP: %s] 删除文件：%s", time.Now().Format(time.RFC3339), c.ClientIP(), name)
		c.String(http.StatusOK, "文件删除成功")
	})

	r.POST("/upload", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			log.Println("无法获取上传的文件:", err)
			c.String(http.StatusBadRequest, "无法获取上传的文件")
			return
		}

		// 保存文件的逻辑
		err = c.SaveUploadedFile(file, "uploads/"+file.Filename)
		if err != nil {
			log.Println("无法保存文件:", err)
			c.String(http.StatusInternalServerError, "无法保存文件")
			return
		}
		log.Printf("[%s] [IP: %s] 上传文件：%s", time.Now().Format(time.RFC3339), c.ClientIP(), file.Filename)
		successHTML := `
		<!DOCTYPE html>
		<html>
		<head>
			<title>上传成功</title>
			<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/twitter-bootstrap/5.0.2/css/bootstrap.min.css">
		</head>
		<body>
			<div class="container">
				<h1>文件上传成功</h1>
				<p>文件名：{{ .Name }}</p>
				<p>文件URL：<a href="{{ .URL }}">{{ .URL }}</a></p>
				<a href="/" class="btn btn-primary">返回首页</a>
			</div>
			<script src="https://cdnjs.cloudflare.com/ajax/libs/twitter-bootstrap/5.0.2/js/bootstrap.bundle.min.js"></script>
		</body>
		</html>
	`
		tmpl, err := template.New("success").Parse(successHTML)
		if err != nil {
			log.Println("无法解析模板:", err)
			c.String(http.StatusInternalServerError, "无法解析模板")
			return
		}

		c.Header("Content-Type", "text/html; charset=utf-8")
		err = tmpl.Execute(c.Writer, File{
			Name: file.Filename,
			URL:  "/uploads/" + file.Filename,
		})
		if err != nil {
			log.Println("无法渲染模板:", err)
			c.String(http.StatusInternalServerError, "无法渲染模板")
			return
		}
	})

	r.Run(":" + strconv.Itoa(config.Port))
}

func getTodayUploadCount() int {
	// 获取今天上传的文件数量的逻辑...
	return 0
}

func getTodayDownloadCount() int {
	// 获取今天下载的文件数量的逻辑...
	return 0
}

func getFileList(dir string) []File {
	files, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		log.Println("无法获取文件列表:", err)
		return nil
	}

	var fileList []File
	for _, file := range files {
		fileName := filepath.Base(file)
		fileURL := "/uploads/" + fileName
		fileInfo := File{
			Name:     fileName,
			URL:      fileURL,
			IsDir:    false,
			Password: "your_password",
		}
		fileList = append(fileList, fileInfo)
	}

	return fileList
}

func deleteFile(name string) error {
	filePath := filepath.Join("uploads", name)
	err := os.Remove(filePath)
	if err != nil {
		log.Println("无法删除文件:", err)
		return err
	}

	return nil
}
