package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// MCPRequest MCP Protocol structures
type MCPRequest struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type MCPResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Tool definitions
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type ToolInputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

// Database structures
type TableInfo struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

type QueryResult struct {
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
	Count   int                      `json:"count"`
}

// MySQL配置
type MySQLConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

type MCPServer struct {
	db     *sql.DB
	config MySQLConfig
}

func NewMCPServer() *MCPServer {
	return &MCPServer{}
}

// 从环境变量或默认值加载配置
func (s *MCPServer) loadConfig() {
	s.config = MySQLConfig{
		Host:     getEnv("MYSQL_HOST", "localhost"),
		Port:     getEnvInt("MYSQL_PORT", 3306),
		User:     getEnv("MYSQL_USER", "root"),
		Password: getEnv("MYSQL_PASSWORD", "Aa130069711"),
		Database: getEnv("MYSQL_DATABASE", "mcp_test"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func (s *MCPServer) initDatabase() error {
	s.loadConfig()

	// 构建MySQL连接字符串
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true",
		s.config.User,
		s.config.Password,
		s.config.Host,
		s.config.Port,
		s.config.Database,
	)

	var err error
	s.db, err = sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("连接数据库失败: %v", err)
	}

	// 测试连接
	if err = s.db.Ping(); err != nil {
		return fmt.Errorf("数据库连接测试失败: %v", err)
	}

	// 创建示例表和数据
	err = s.createSampleTables()
	if err != nil {
		log.Printf("创建示例表失败: %v", err)
		// 不返回错误，允许使用现有数据库
	}

	return nil
}

func (s *MCPServer) createSampleTables() error {
	// 创建users表
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			email VARCHAR(100) UNIQUE NOT NULL,
			age INT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`)
	if err != nil {
		return err
	}

	// 创建orders表
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT,
			product_name VARCHAR(200) NOT NULL,
			amount DECIMAL(10,2),
			status VARCHAR(50) DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`)
	if err != nil {
		return err
	}

	// 插入示例数据 (使用INSERT IGNORE避免重复)
	_, err = s.db.Exec(`
		INSERT IGNORE INTO users (id, name, email, age) VALUES 
		(1, 'Alice Smith', 'alice@example.com', 30),
		(2, 'Bob Johnson', 'bob@example.com', 25),
		(3, 'Carol Brown', 'carol@example.com', 35)
	`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		INSERT IGNORE INTO orders (id, user_id, product_name, amount, status) VALUES 
		(1, 1, 'Laptop', 999.99, 'completed'),
		(2, 1, 'Mouse', 29.99, 'pending'),
		(3, 2, 'Keyboard', 79.99, 'completed'),
		(4, 3, 'Monitor', 299.99, 'shipped')
	`)

	return err
}

func (s *MCPServer) handleRequest(req MCPRequest) MCPResponse {
	switch req.Method {
	case "initialize":
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]interface{}{
					"name":    "mysql-mcp-server",
					"version": "1.0.0",
				},
			},
		}

	case "tools/list":
		tools := []Tool{
			{
				Name:        "list_tables",
				Description: "列出数据库中的所有表",
				InputSchema: ToolInputSchema{
					Type:       "object",
					Properties: map[string]interface{}{},
				},
			},
			{
				Name:        "describe_table",
				Description: "获取指定表的结构信息",
				InputSchema: ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"table_name": map[string]interface{}{
							"type":        "string",
							"description": "表名",
						},
					},
					Required: []string{"table_name"},
				},
			},
			{
				Name:        "query_table",
				Description: "查询表数据",
				InputSchema: ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"table_name": map[string]interface{}{
							"type":        "string",
							"description": "表名",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "限制返回行数，默认10",
						},
						"where_clause": map[string]interface{}{
							"type":        "string",
							"description": "WHERE条件子句（可选）",
						},
					},
					Required: []string{"table_name"},
				},
			},
			{
				Name:        "execute_query",
				Description: "执行自定义SQL查询（仅SELECT语句）",
				InputSchema: ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "SQL查询语句",
						},
					},
					Required: []string{"query"},
				},
			},
			{
				Name:        "show_table_indexes",
				Description: "显示表的索引信息",
				InputSchema: ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"table_name": map[string]interface{}{
							"type":        "string",
							"description": "表名",
						},
					},
					Required: []string{"table_name"},
				},
			},
		}

		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"tools": tools,
			},
		}

	case "tools/call":
		return s.handleToolCall(req)

	default:
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

func (s *MCPServer) handleToolCall(req MCPRequest) MCPResponse {
	var params struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return MCPResponse{
			Jsonrpc: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}

	switch params.Name {
	case "list_tables":
		return s.listTables(req.ID)
	case "describe_table":
		tableName, ok := params.Arguments["table_name"].(string)
		if !ok {
			return s.errorResponse(req.ID, "table_name is required")
		}
		return s.describeTable(req.ID, tableName)
	case "query_table":
		return s.queryTable(req.ID, params.Arguments)
	case "execute_query":
		query, ok := params.Arguments["query"].(string)
		if !ok {
			return s.errorResponse(req.ID, "query is required")
		}
		return s.executeQuery(req.ID, query)
	case "show_table_indexes":
		tableName, ok := params.Arguments["table_name"].(string)
		if !ok {
			return s.errorResponse(req.ID, "table_name is required")
		}
		return s.showTableIndexes(req.ID, tableName)
	default:
		return s.errorResponse(req.ID, "Unknown tool")
	}
}

func (s *MCPServer) listTables(id interface{}) MCPResponse {
	rows, err := s.db.Query("SHOW TABLES")
	if err != nil {
		return s.errorResponse(id, fmt.Sprintf("Database error: %v", err))
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}
		tables = append(tables, tableName)
	}

	return MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("数据库 '%s' 中的表: %s", s.config.Database, strings.Join(tables, ", ")),
				},
			},
		},
	}
}

func (s *MCPServer) describeTable(id interface{}, tableName string) MCPResponse {
	rows, err := s.db.Query("DESCRIBE " + tableName)
	if err != nil {
		return s.errorResponse(id, fmt.Sprintf("Database error: %v", err))
	}
	defer rows.Close()

	result := fmt.Sprintf("表 '%s' 的结构:\n\n", tableName)
	result += fmt.Sprintf("%-20s %-20s %-10s %-10s %-15s %-10s\n",
		"字段名", "数据类型", "是否为空", "键", "默认值", "额外信息")
	result += strings.Repeat("-", 90) + "\n"

	for rows.Next() {
		var field, dataType, null, key string
		var defaultValue, extra interface{}

		err := rows.Scan(&field, &dataType, &null, &key, &defaultValue, &extra)
		if err != nil {
			continue
		}

		defaultStr := "NULL"
		if defaultValue != nil {
			defaultStr = fmt.Sprintf("%v", defaultValue)
		}

		extraStr := ""
		if extra != nil {
			extraStr = fmt.Sprintf("%v", extra)
		}

		result += fmt.Sprintf("%-20s %-20s %-10s %-10s %-15s %-10s\n",
			field, dataType, null, key, defaultStr, extraStr)
	}

	return MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": result,
				},
			},
		},
	}
}

func (s *MCPServer) showTableIndexes(id interface{}, tableName string) MCPResponse {
	rows, err := s.db.Query("SHOW INDEX FROM " + tableName)
	if err != nil {
		return s.errorResponse(id, fmt.Sprintf("Database error: %v", err))
	}
	defer rows.Close()

	result := fmt.Sprintf("表 '%s' 的索引信息:\n\n", tableName)
	result += fmt.Sprintf("%-20s %-15s %-15s %-15s %-10s\n",
		"索引名", "列名", "是否唯一", "索引类型", "序列")
	result += strings.Repeat("-", 80) + "\n"

	for rows.Next() {
		var table, nonUnique, keyName, seqInIndex, columnName string
		var collation, cardinality, subPart, packed, null, indexType, comment, indexComment interface{}

		err := rows.Scan(&table, &nonUnique, &keyName, &seqInIndex, &columnName,
			&collation, &cardinality, &subPart, &packed, &null, &indexType, &comment, &indexComment)
		if err != nil {
			continue
		}

		unique := "否"
		if nonUnique == "0" {
			unique = "是"
		}

		indexTypeStr := ""
		if indexType != nil {
			indexTypeStr = fmt.Sprintf("%v", indexType)
		}

		result += fmt.Sprintf("%-20s %-15s %-15s %-15s %-10s\n",
			keyName, columnName, unique, indexTypeStr, seqInIndex)
	}

	return MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": result,
				},
			},
		},
	}
}

func (s *MCPServer) queryTable(id interface{}, args map[string]interface{}) MCPResponse {
	tableName, ok := args["table_name"].(string)
	if !ok {
		return s.errorResponse(id, "table_name is required")
	}

	limit := 10
	if l, ok := args["limit"]; ok {
		if lf, ok := l.(float64); ok {
			limit = int(lf)
		}
	}

	query := fmt.Sprintf("SELECT * FROM `%s`", tableName)

	if whereClause, ok := args["where_clause"].(string); ok && whereClause != "" {
		query += " WHERE " + whereClause
	}

	query += " LIMIT " + strconv.Itoa(limit)

	return s.executeQuery(id, query)
}

func (s *MCPServer) executeQuery(id interface{}, query string) MCPResponse {
	// 安全检查：只允许SELECT语句和SHOW语句
	upperQuery := strings.ToUpper(strings.TrimSpace(query))
	if !strings.HasPrefix(upperQuery, "SELECT") &&
		!strings.HasPrefix(upperQuery, "SHOW") &&
		!strings.HasPrefix(upperQuery, "DESCRIBE") &&
		!strings.HasPrefix(upperQuery, "DESC") {
		return s.errorResponse(id, "只允许执行SELECT、SHOW、DESCRIBE查询")
	}

	rows, err := s.db.Query(query)
	if err != nil {
		return s.errorResponse(id, fmt.Sprintf("查询错误: %v", err))
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return s.errorResponse(id, fmt.Sprintf("获取列信息错误: %v", err))
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	// 格式化输出
	resultText := fmt.Sprintf("查询结果 (%d 行):\n\n", len(results))
	if len(results) > 0 {
		// 计算每列的最大宽度
		colWidths := make(map[string]int)
		for _, col := range columns {
			colWidths[col] = len(col)
		}
		for _, row := range results {
			for _, col := range columns {
				value := row[col]
				valueStr := "NULL"
				if value != nil {
					valueStr = fmt.Sprintf("%v", value)
				}
				if len(valueStr) > colWidths[col] {
					colWidths[col] = len(valueStr)
				}
			}
		}

		// 表头
		for _, col := range columns {
			width := colWidths[col]
			if width < 8 {
				width = 8
			}
			if width > 30 {
				width = 30
			}
			resultText += fmt.Sprintf("%-*s ", width, col)
		}
		resultText += "\n"

		// 分隔线
		totalWidth := 0
		for _, col := range columns {
			width := colWidths[col]
			if width < 8 {
				width = 8
			}
			if width > 30 {
				width = 30
			}
			totalWidth += width + 1
		}
		resultText += strings.Repeat("-", totalWidth) + "\n"

		// 数据行
		for _, row := range results {
			for _, col := range columns {
				width := colWidths[col]
				if width < 8 {
					width = 8
				}
				if width > 30 {
					width = 30
				}

				value := row[col]
				valueStr := "NULL"
				if value != nil {
					valueStr = fmt.Sprintf("%v", value)
					if len(valueStr) > 30 {
						valueStr = valueStr[:27] + "..."
					}
				}
				resultText += fmt.Sprintf("%-*s ", width, valueStr)
			}
			resultText += "\n"
		}
	} else {
		resultText += "没有找到数据\n"
	}

	return MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": resultText,
				},
			},
		},
	}
}

func (s *MCPServer) errorResponse(id interface{}, message string) MCPResponse {
	return MCPResponse{
		Jsonrpc: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    -32603,
			Message: message,
		},
	}
}

func (s *MCPServer) run() {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var req MCPRequest
		if err := decoder.Decode(&req); err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Printf("解码请求错误: %v", err)
			continue
		}

		response := s.handleRequest(req)
		if err := encoder.Encode(response); err != nil {
			log.Printf("编码响应错误: %v", err)
		}
	}
}

func main() {
	server := NewMCPServer()

	if err := server.initDatabase(); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer server.db.Close()

	log.Printf("MySQL MCP Server 启动...")
	log.Printf("连接到: %s:%d/%s", server.config.Host, server.config.Port, server.config.Database)
	server.run()
}
