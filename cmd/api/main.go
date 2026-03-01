package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	godotenv.Load()

	// Set Gin to release mode
	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"timestamp": time.Now().Unix(),
		})
	})

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Scans
		v1.GET("/scans", getScans)
		v1.GET("/scans/:id", getScan)
		v1.POST("/scans", createScan)
		v1.DELETE("/scans/:id", deleteScan)
		v1.POST("/scans/:id/run", runScan)

		// AI
		v1.POST("/ai/chat", aiChat)
		v1.POST("/ai/validate", aiValidate)
		v1.POST("/ai/review", aiReview)
		v1.POST("/ai/explain-error", aiExplainError)

		// Etherscan
		v1.GET("/etherscan/balance/:address", getEthBalance)
		v1.GET("/etherscan/tx/:txHash", getTxReceipt)
		v1.GET("/etherscan/txlist/:address", getTxList)
		v1.GET("/etherscan/abi/:address", getContractABI)
		v1.GET("/etherscan/sourcecode/:address", getSourceCode)

		// GitHub
		v1.GET("/github/repos/:owner/:repo", getRepo)
		v1.GET("/github/repos/:owner/:repo/issues", getIssues)
		v1.POST("/github/repos/:owner/:repo/pulls", createPR)
		v1.GET("/github/repos/:owner/:repo/actions/runs", getWorkflowRuns)
		v1.POST("/github/repos/:owner/:repo/actions/workflows/:workflow_id/dispatch", triggerWorkflow)
		// Additional endpoints for frontend
		v1.GET("/github/workflows/:repo", getWorkflowsByName)
		v1.GET("/github/runs/:repo", getRunsByRepo)
		v1.GET("/github/prs/:repo", getPRsByRepo)
		v1.GET("/github/actions/:repo", getActionsByRepo)
		v1.GET("/github/repos", getAllRepos)
	}

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

// Scan types
type Scan struct {
	ID        string    `json:"id"`
	Target    string    `json:"target"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	Result    string    `json:"result,omitempty"`
}

var scans = make(map[string]Scan)

// Scan handlers
func getScans(c *gin.Context) {
	result := make([]Scan, 0, len(scans))
	for _, s := range scans {
		result = append(result, s)
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func getScan(c *gin.Context) {
	id := c.Param("id")
	if s, ok := scans[id]; ok {
		c.JSON(http.StatusOK, gin.H{"data": s})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "scan not found"})
	}
}

func createScan(c *gin.Context) {
	var input struct {
		Target string `json:"target" binding:"required"`
		Type   string `json:"type"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id := fmt.Sprintf("scan_%d", len(scans)+1)
	scan := Scan{
		ID:        id,
		Target:    input.Target,
		Type:      input.Type,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	scans[id] = scan
	c.JSON(http.StatusCreated, gin.H{"data": scan})
}

func deleteScan(c *gin.Context) {
	id := c.Param("id")
	if _, ok := scans[id]; ok {
		delete(scans, id)
		c.JSON(http.StatusOK, gin.H{"message": "deleted"})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	}
}

func runScan(c *gin.Context) {
	id := c.Param("id")
	if s, ok := scans[id]; ok {
		s.Status = "running"
		scans[id] = s
		c.JSON(http.StatusOK, gin.H{"data": s, "message": "scan started"})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	}
}

// AI handlers
func aiChat(c *gin.Context) {
	var input struct {
		Message string `json:"message" binding:"required"`
		Model   string `json:"model"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reply": "AI response: " + input.Message})
}

func aiValidate(c *gin.Context) {
	var input struct {
		Code string `json:"code" binding:"required"`
		Model string `json:"model"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"valid": true, "issues": []string{}})
}

func aiReview(c *gin.Context) {
	var input struct {
		Code string `json:"code" binding:"required"`
		Model string `json:"model"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"review": "Code looks good", "score": 85})
}

func aiExplainError(c *gin.Context) {
	var input struct {
		Error string `json:"error" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"explanation": "Error explanation here"})
}

// Etherscan handlers
func getEthBalance(c *gin.Context) {
	address := c.Param("address")
	c.JSON(http.StatusOK, gin.H{
		"address": address,
		"balance": "0",
		"symbol":  "ETH",
	})
}

func getTxReceipt(c *gin.Context) {
	txHash := c.Param("txHash")
	c.JSON(http.StatusOK, gin.H{
		"hash":         txHash,
		"blockNumber":  "12345678",
		"from":         "0x0000000000000000000000000000000000000000",
		"to":           "0x0000000000000000000000000000000000000000",
		"status":       "1",
	})
}

func getTxList(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []struct{}{}, "message": "OK"})
}

func getContractABI(c *gin.Context) {
	address := c.Param("address")
	c.JSON(http.StatusOK, gin.H{"address": address, "abi": "[]"})
}

func getSourceCode(c *gin.Context) {
	address := c.Param("address")
	c.JSON(http.StatusOK, gin.H{"address": address, "sourceCode": "// Contract source code"})
}

// GitHub handlers
func getRepo(c *gin.Context) {
	owner := c.Param("owner")
	repo := c.Param("repo")
	c.JSON(http.StatusOK, gin.H{
		"name":         repo,
		"full_name":    owner + "/" + repo,
		"owner":        gin.H{"login": owner},
		"description":  "Repository",
		"language":     "Go",
		"stargazers_count": 0,
		"forks_count": 0,
		"open_issues_count": 0,
	})
}

func getIssues(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []struct{}{}})
}

func createPR(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{"message": "PR created"})
}

func getWorkflowRuns(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": []struct{}{}})
}

func triggerWorkflow(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "workflow triggered"})
}

// Additional GitHub handlers for frontend compatibility
func getAllRepos(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": []gin.H{
			{
				"name":         "vespera-coze",
				"full_name":    "jssyxd/vespera-coze",
				"visibility":   "public",
				"language":      "Go",
				"stars":         0,
			},
		},
	})
}

func getWorkflowsByName(c *gin.Context) {
	repo := c.Param("repo")
	c.JSON(http.StatusOK, gin.H{
		"data": []gin.H{
			{"id": 1, "name": "CI", "state": "active"},
			{"id": 2, "name": "Deploy", "state": "active"},
		},
		"repo": repo,
	})
}

func getRunsByRepo(c *gin.Context) {
	repo := c.Param("repo")
	c.JSON(http.StatusOK, gin.H{
		"data": []gin.H{},
		"repo": repo,
	})
}

func getPRsByRepo(c *gin.Context) {
	repo := c.Param("repo")
	c.JSON(http.StatusOK, gin.H{
		"data": []gin.H{},
		"repo": repo,
	})
}

func getActionsByRepo(c *gin.Context) {
	repo := c.Param("repo")
	c.JSON(http.StatusOK, gin.H{
		"data": []gin.H{},
		"repo": repo,
	})
}
