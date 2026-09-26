package dorks

import (
	"fmt"
	"net/url"
	"strings"
)

// DorkItem represents a single security recon search query
type DorkItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Query       string `json:"query"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // "high", "medium", "info"
	Category    string `json:"category"`
}

// DorkCategory represents a group of related dorks
type DorkCategory struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Icon        string     `json:"icon"`
	Description string     `json:"description"`
	Dorks       []DorkItem `json:"dorks"`
}

// BuildQuery adapts a dork query to an optional target domain
func (d *DorkItem) BuildQuery(targetDomain string) string {
	target := strings.TrimSpace(targetDomain)
	target = strings.TrimPrefix(target, "http://")
	target = strings.TrimPrefix(target, "https://")
	if idx := strings.Index(target, "/"); idx != -1 {
		target = target[:idx]
	}

	if target == "" {
		return d.Query
	}

	// Avoid duplicate site: operator if already present
	if strings.Contains(strings.ToLower(d.Query), "site:") {
		return d.Query
	}

	return fmt.Sprintf("site:%s %s", target, d.Query)
}

// GoogleURL generates a direct Google search link
func (d *DorkItem) GoogleURL(targetDomain string) string {
	q := d.BuildQuery(targetDomain)
	return fmt.Sprintf("https://www.google.com/search?q=%s", url.QueryEscape(q))
}

// SearxgoURL generates an internal SearXGo search link
func (d *DorkItem) SearxgoURL(targetDomain string) string {
	q := d.BuildQuery(targetDomain)
	return fmt.Sprintf("/search?q=%s&category=general", url.QueryEscape(q))
}

// GetAllCategories returns the curated database of security recon dorks
func GetAllCategories() []DorkCategory {
	return []DorkCategory{
		{
			ID:          "configs",
			Name:        "Secrets & Configs",
			Icon:        "🔑",
			Description: "Exposed environment files, API tokens, cloud credentials, and private keys.",
			Dorks: []DorkItem{
				{
					ID:          "cfg-env",
					Title:       "Exposed .env Credentials",
					Query:       `filetype:env "DB_PASSWORD" OR "APP_KEY" OR "SECRET_KEY"`,
					Description: "Finds publicly accessible dotenv files containing database credentials and app secrets.",
					Severity:    "high",
					Category:    "configs",
				},
				{
					ID:          "cfg-aws",
					Title:       "AWS / Cloud Credentials",
					Query:       `filetype:json "aws_secret_access_key" OR "private_key_id"`,
					Description: "Discovers exposed cloud service accounts, AWS credentials, and GCP service keys.",
					Severity:    "high",
					Category:    "configs",
				},
				{
					ID:          "cfg-git",
					Title:       "Exposed .git Folder",
					Query:       `inurl:"/.git/config" OR inurl:"/.git/HEAD"`,
					Description: "Detects misconfigured web servers exposing the entire Git repository history and source code.",
					Severity:    "high",
					Category:    "configs",
				},
				{
					ID:          "cfg-ssh",
					Title:       "SSH Private Keys",
					Query:       `intext:"BEGIN RSA PRIVATE KEY" OR intext:"BEGIN OPENSSH PRIVATE KEY"`,
					Description: "Identifies leaked RSA and OpenSSH private encryption keys.",
					Severity:    "high",
					Category:    "configs",
				},
				{
					ID:          "cfg-docker",
					Title:       "Docker & Kubernetes Configs",
					Query:       `filename:docker-compose.yml "POSTGRES_PASSWORD" OR filename:kubeconfig`,
					Description: "Detects container and cluster manifests containing hardcoded database credentials.",
					Severity:    "medium",
					Category:    "configs",
				},
			},
		},
		{
			ID:          "opendir",
			Name:        "Open Directories",
			Icon:        "📂",
			Description: "Unrestricted directory listings, backup tarballs, and archived storage.",
			Dorks: []DorkItem{
				{
					ID:          "dir-index",
					Title:       "Web Server Directory Index",
					Query:       `intitle:"index of /" "parent directory"`,
					Description: "Locates web servers with auto-indexing enabled exposing underlying folder structures.",
					Severity:    "medium",
					Category:    "opendir",
				},
				{
					ID:          "dir-backup",
					Title:       "Exposed Backup Archives",
					Query:       `intitle:"index of" (backup.zip OR backup.tar.gz OR site_backup.sql)`,
					Description: "Finds forgotten full-site backup archives, database snapshots, and zipped source code.",
					Severity:    "high",
					Category:    "opendir",
				},
				{
					ID:          "dir-logs",
					Title:       "Server Log Files",
					Query:       `intitle:"index of /" "access.log" OR "error.log" OR "debug.log"`,
					Description: "Discovers open server access logs exposing visitor IP addresses, query strings, and system errors.",
					Severity:    "medium",
					Category:    "opendir",
				},
				{
					ID:          "dir-ftp",
					Title:       "Open FTP Directories",
					Query:       `intitle:"index of /" inurl:ftp`,
					Description: "Finds anonymous or publicly readable FTP directory mirrors.",
					Severity:    "medium",
					Category:    "opendir",
				},
			},
		},
		{
			ID:          "admin",
			Name:        "Admin & Portals",
			Icon:        "🚪",
			Description: "Exposed management consoles, database GUIs, and router dashboards.",
			Dorks: []DorkItem{
				{
					ID:          "adm-login",
					Title:       "CMS & Web Admin Logins",
					Query:       `inurl:admin/login.php OR inurl:wp-login.php OR inurl:administrator/index.php`,
					Description: "Discovers authentication endpoints for WordPress, Joomla, Drupal, and custom CMS portals.",
					Severity:    "medium",
					Category:    "admin",
				},
				{
					ID:          "adm-db",
					Title:       "phpMyAdmin & Adminer",
					Query:       `inurl:phpmyadmin/index.php OR inurl:adminer.php intitle:"Login"`,
					Description: "Locates publicly exposed database management consoles.",
					Severity:    "high",
					Category:    "admin",
				},
				{
					ID:          "adm-grafana",
					Title:       "Grafana & Kibana Dashboards",
					Query:       `intitle:"Grafana - Home" OR intitle:"Kibana - Discover" inurl:app/kibana`,
					Description: "Detects unauthenticated telemetry dashboards and log analytics consoles.",
					Severity:    "medium",
					Category:    "admin",
				},
				{
					ID:          "adm-cpanel",
					Title:       "cPanel & Webmin Ports",
					Query:       `inurl:":2083" OR inurl:":10000" intitle:"Webmin Login"`,
					Description: "Identifies exposed web hosting control panels.",
					Severity:    "medium",
					Category:    "admin",
				},
			},
		},
		{
			ID:          "database",
			Name:        "Database Dumps",
			Icon:        "🗄️",
			Description: "Direct SQL dumps, database tables, and SQLite/JSON databases.",
			Dorks: []DorkItem{
				{
					ID:          "db-sql",
					Title:       "Raw SQL Export Dumps",
					Query:       `filetype:sql ("INSERT INTO" AND "password") OR "CREATE TABLE users"`,
					Description: "Detects exposed SQL database export files containing user tables and hashed credentials.",
					Severity:    "high",
					Category:    "database",
				},
				{
					ID:          "db-sqlite",
					Title:       "SQLite Database Files",
					Query:       `filetype:sqlite OR filetype:db "SQLite format 3"`,
					Description: "Finds downloadable SQLite database files used by desktop apps and embedded services.",
					Severity:    "high",
					Category:    "database",
				},
				{
					ID:          "db-mongo",
					Title:       "JSON & MongoDB Collections",
					Query:       `filetype:json ("_id" AND "password_hash" OR "hashedPassword")`,
					Description: "Discovers exported MongoDB collections and JSON user data stores.",
					Severity:    "high",
					Category:    "database",
				},
			},
		},
		{
			ID:          "docs",
			Name:        "Sensitive Documents",
			Icon:        "📄",
			Description: "Confidential agreements, payrolls, internal memos, and NDAs.",
			Dorks: []DorkItem{
				{
					ID:          "doc-confidential",
					Title:       "Confidential & Internal PDFs",
					Query:       `filetype:pdf ("CONFIDENTIAL" OR "INTERNAL USE ONLY" OR "STRICTLY PRIVATE")`,
					Description: "Locates leaked corporate documents labeled as strictly confidential.",
					Severity:    "high",
					Category:    "docs",
				},
				{
					ID:          "doc-payroll",
					Title:       "Salary & Payroll Spreadsheets",
					Query:       `filetype:xlsx OR filetype:csv ("salary" OR "payroll" OR "bonus" OR "gross pay")`,
					Description: "Finds exposed human resources spreadsheets containing employee compensation details.",
					Severity:    "high",
					Category:    "docs",
				},
				{
					ID:          "doc-passwords",
					Title:       "Password Lists & Cleartext Credentials",
					Query:       `filetype:txt OR filetype:docx ("username" AND "password" AND "login")`,
					Description: "Discovers plain text files and notes containing system passwords.",
					Severity:    "high",
					Category:    "docs",
				},
			},
		},
		{
			ID:          "vulns",
			Name:        "Attack Surface & APIs",
			Icon:        "🐞",
			Description: "Swagger specs, phpinfo outputs, stack traces, and debug endpoints.",
			Dorks: []DorkItem{
				{
					ID:          "vuln-phpinfo",
					Title:       "Live phpinfo() Diagnostic",
					Query:       `intitle:"phpinfo()" "PHP Version" "System" "Loaded Configuration File"`,
					Description: "Discloses exact PHP version, enabled modules, operating system, and internal file paths.",
					Severity:    "medium",
					Category:    "vulns",
				},
				{
					ID:          "vuln-swagger",
					Title:       "Swagger / OpenAPI Docs",
					Query:       `inurl:"/swagger-ui.html" OR inurl:"/api-docs" OR inurl:"/swagger/v1/swagger.json"`,
					Description: "Detects exposed interactive REST API documentation and hidden internal endpoints.",
					Severity:    "info",
					Category:    "vulns",
				},
				{
					ID:          "vuln-actuator",
					Title:       "Spring Boot Actuator",
					Query:       `inurl:"/actuator/env" OR inurl:"/actuator/heapdump"`,
					Description: "Finds vulnerable Java Spring Boot actuator endpoints leaking environment variables.",
					Severity:    "high",
					Category:    "vulns",
				},
				{
					ID:          "vuln-stacktrace",
					Title:       "Detailed Error Stack Traces",
					Query:       `"Fatal error:" OR "Uncaught exception" inurl:".php"`,
					Description: "Locates unhandled web application errors exposing line numbers, SQL queries, and source code.",
					Severity:    "medium",
					Category:    "vulns",
				},
			},
		},
		{
			ID:          "cloud",
			Name:        "Cloud Buckets",
			Icon:        "☁️",
			Description: "Misconfigured public cloud buckets on AWS S3, Azure Blob, and Firebase.",
			Dorks: []DorkItem{
				{
					ID:          "cld-s3",
					Title:       "AWS S3 Public Buckets",
					Query:       `site:s3.amazonaws.com OR site:storage.googleapis.com`,
					Description: "Discovers public cloud storage buckets containing company assets and backups.",
					Severity:    "medium",
					Category:    "cloud",
				},
				{
					ID:          "cld-firebase",
					Title:       "Open Firebase Databases",
					Query:       `site:firebaseio.com inurl:.json`,
					Description: "Detects unauthenticated Firebase real-time databases dumping complete app state.",
					Severity:    "high",
					Category:    "cloud",
				},
			},
		},
		{
			ID:          "iot",
			Name:        "IoT & Network Devices",
			Icon:        "📹",
			Description: "Live IP webcams, network printers, VoIP systems, and PBX switches.",
			Dorks: []DorkItem{
				{
					ID:          "iot-cams",
					Title:       "Live Video Webcams",
					Query:       `intitle:"Live View / - AXIS" OR inurl:"view/view.shtml" OR intitle:"Network Camera NetworkCamera"`,
					Description: "Finds public network surveillance cameras streaming video without authentication.",
					Severity:    "medium",
					Category:    "iot",
				},
				{
					ID:          "iot-printers",
					Title:       "Networked Office Printers",
					Query:       `intitle:"Network Print Server" OR inurl:"hp/device/this.LCDispatcher"`,
					Description: "Locates exposed enterprise network printer consoles and queue management pages.",
					Severity:    "medium",
					Category:    "iot",
				},
			},
		},
	}
}
