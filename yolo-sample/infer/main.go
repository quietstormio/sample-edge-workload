package main

import (
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Detection struct {
	ClassID    int     `json:"class_id"`
	ClassName  string  `json:"class_name"`
	Confidence float64 `json:"confidence"`
	BBox       BBox    `json:"bbox"`
}

type BBox struct {
	X1 float64 `json:"x1"`
	Y1 float64 `json:"y1"`
	X2 float64 `json:"x2"`
	Y2 float64 `json:"y2"`
}

type InferenceResult struct {
	Image      string      `json:"image"`
	Detections []Detection `json:"detections"`
	Count      int         `json:"count"`
	Error      string      `json:"error,omitempty"`
}

type SystemStatus struct {
	NetworkStatus  string // "online", "offline", or "unknown"
	TrainingEnabled bool
}

type PageData struct {
	Status SystemStatus
}

type ResultPageData struct {
	Status SystemStatus
	Result InferenceResult
}

var uploadDir = "/tmp/uploads"

// getNodeStatus queries the node's network-status label using kubectl
func getNodeStatus() SystemStatus {
	log.Println("DEBUG: getNodeStatus() called")
	nodeName := os.Getenv("NODE_NAME")
	labelKey := os.Getenv("NODE_LABEL_KEY")

	log.Printf("DEBUG: NODE_NAME=%s, NODE_LABEL_KEY=%s", nodeName, labelKey)

	if nodeName == "" || labelKey == "" {
		log.Println("Warning: NODE_NAME or NODE_LABEL_KEY not set, defaulting to unknown status")
		return SystemStatus{NetworkStatus: "unknown", TrainingEnabled: false}
	}

	// Use kubectl to get the node label
	// Escape dots in the label key for jsonpath (e.g., myapp.com becomes myapp\.com)
	// Forward slashes don't need escaping
	escapedLabelKey := strings.ReplaceAll(labelKey, ".", "\\.")
	jsonPath := "jsonpath={.metadata.labels." + escapedLabelKey + "}"
	log.Printf("DEBUG: Running kubectl command: kubectl get node %s -o %s", nodeName, jsonPath)

	cmd := exec.Command("kubectl", "get", "node", nodeName, "-o", jsonPath)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Warning: Failed to get node status: %v", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Printf("DEBUG: stderr: %s", string(exitErr.Stderr))
		}
		return SystemStatus{NetworkStatus: "unknown", TrainingEnabled: false}
	}

	status := strings.TrimSpace(string(output))
	log.Printf("DEBUG: kubectl returned: '%s'", status)

	if status == "" {
		log.Println("DEBUG: Status is empty, setting to unknown")
		status = "unknown"
	}

	trainingEnabled := status == "online"

	log.Printf("DEBUG: Final status - NetworkStatus: %s, TrainingEnabled: %t", status, trainingEnabled)

	return SystemStatus{
		NetworkStatus:  status,
		TrainingEnabled: trainingEnabled,
	}
}

func main() {
	// Create upload directory
	os.MkdirAll(uploadDir, 0755)

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/upload", uploadHandler)

	log.Println("Starting YOLO Inference Web UI on :6767")
	log.Fatal(http.ListenAndServe(":6767", nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	status := getNodeStatus()

	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>YOLO Inference</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        h1 {
            color: #333;
            text-align: center;
        }
        .upload-form {
            background: white;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        input[type="file"] {
            margin: 20px 0;
            padding: 10px;
        }
        button {
            background-color: #4CAF50;
            color: white;
            padding: 12px 30px;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 16px;
        }
        button:hover {
            background-color: #45a049;
        }
        .results {
            margin-top: 30px;
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .detection {
            padding: 10px;
            margin: 10px 0;
            background-color: #e8f5e9;
            border-left: 4px solid #4CAF50;
        }
        .error {
            color: #d32f2f;
            background-color: #ffebee;
            padding: 15px;
            border-radius: 4px;
            border-left: 4px solid #d32f2f;
        }
        .status-bar {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            padding: 20px 30px;
            border-radius: 8px;
            box-shadow: 0 4px 12px rgba(102, 126, 234, 0.3);
            margin-bottom: 20px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .status-item {
            display: flex;
            align-items: center;
            gap: 12px;
        }
        .status-indicator {
            width: 14px;
            height: 14px;
            border-radius: 50%;
            display: inline-block;
            box-shadow: 0 0 8px rgba(255,255,255,0.5);
        }
        .status-indicator.online {
            background-color: #4CAF50;
            animation: pulse-green 2s infinite;
        }
        .status-indicator.offline {
            background-color: #f44336;
            animation: pulse-red 2s infinite;
        }
        .status-indicator.unknown {
            background-color: #ff9800;
            animation: pulse-orange 2s infinite;
        }
        @keyframes pulse-green {
            0%, 100% { box-shadow: 0 0 8px rgba(76, 175, 80, 0.4); }
            50% { box-shadow: 0 0 16px rgba(76, 175, 80, 0.8); }
        }
        @keyframes pulse-red {
            0%, 100% { box-shadow: 0 0 8px rgba(244, 67, 54, 0.4); }
            50% { box-shadow: 0 0 16px rgba(244, 67, 54, 0.8); }
        }
        @keyframes pulse-orange {
            0%, 100% { box-shadow: 0 0 8px rgba(255, 152, 0, 0.4); }
            50% { box-shadow: 0 0 16px rgba(255, 152, 0, 0.8); }
        }
        .status-label {
            font-weight: bold;
            font-size: 15px;
            color: white;
            text-shadow: 0 1px 2px rgba(0,0,0,0.2);
        }
        .training-status {
            font-size: 14px;
            color: rgba(255,255,255,0.95);
            font-weight: 500;
        }
        .spinner-overlay {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0,0,0,0.7);
            z-index: 9999;
            justify-content: center;
            align-items: center;
            flex-direction: column;
        }
        .spinner-overlay.active {
            display: flex;
        }
        .spinner {
            border: 6px solid #f3f3f3;
            border-top: 6px solid #667eea;
            border-radius: 50%;
            width: 60px;
            height: 60px;
            animation: spin 1s linear infinite;
        }
        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
        .spinner-text {
            color: white;
            margin-top: 20px;
            font-size: 18px;
            font-weight: bold;
        }
        .manual-train-btn {
            background-color: #9e9e9e;
            color: white;
            padding: 10px 20px;
            border: none;
            border-radius: 4px;
            cursor: not-allowed;
            font-size: 14px;
            margin-top: 20px;
            opacity: 0.6;
        }
    </style>
</head>
<body>
    <h1>YOLO Object Detection</h1>
    <div class="status-bar">
        <div class="status-item">
            <span class="status-indicator {{.Status.NetworkStatus}}"></span>
            <span class="status-label">Network: {{.Status.NetworkStatus}}</span>
        </div>
        <div class="status-item">
            <span class="training-status">Training: {{if .Status.TrainingEnabled}}✓ Enabled{{else}}✗ Disabled{{end}}</span>
        </div>
    </div>
    <div class="upload-form">
        <h2>Upload an Image</h2>
        <form action="/upload" method="post" enctype="multipart/form-data" id="uploadForm">
            <input type="file" name="image" accept="image/*" required>
            <br>
            <button type="submit">Run Inference</button>
        </form>
        <button class="manual-train-btn" disabled title="Coming soon: Manual training trigger">
            Trigger Training (Disabled)
        </button>
    </div>

    <!-- Spinner overlay -->
    <div class="spinner-overlay" id="spinnerOverlay">
        <div class="spinner"></div>
        <div class="spinner-text">Running inference...</div>
    </div>

    <script>
        document.getElementById('uploadForm').addEventListener('submit', function() {
            document.getElementById('spinnerOverlay').classList.add('active');
        });
    </script>
</body>
</html>
`
	t, err := template.New("home").Parse(tmpl)
	if err != nil {
		log.Printf("Template parse error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := PageData{Status: status}
	t.Execute(w, data)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form
	err := r.ParseMultipartForm(10 << 20) // 10 MB max
	if err != nil {
		renderError(w, "Failed to parse form: "+err.Error())
		return
	}

	// Get uploaded file
	file, handler, err := r.FormFile("image")
	if err != nil {
		renderError(w, "Failed to get image: "+err.Error())
		return
	}
	defer file.Close()

	// Save file to disk
	filePath := filepath.Join(uploadDir, handler.Filename)
	dst, err := os.Create(filePath)
	if err != nil {
		renderError(w, "Failed to save image: "+err.Error())
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		renderError(w, "Failed to write image: "+err.Error())
		return
	}

	// Run inference
	result := runInference(filePath)

	// Get current system status
	status := getNodeStatus()

	// Render results
	renderResults(w, status, result)
}

func runInference(imagePath string) InferenceResult {
	cmd := exec.Command("python", "/app/infer.py", imagePath)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return InferenceResult{Error: "Inference failed: " + err.Error() + "\n" + string(output)}
	}

	var result InferenceResult
	err = json.Unmarshal(output, &result)
	if err != nil {
		return InferenceResult{Error: "Failed to parse results: " + err.Error()}
	}

	return result
}

func renderError(w http.ResponseWriter, errorMsg string) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Error - YOLO Inference</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
        }
        .error {
            color: #d32f2f;
            background-color: #ffebee;
            padding: 20px;
            border-radius: 4px;
            border-left: 4px solid #d32f2f;
        }
        a {
            display: inline-block;
            margin-top: 20px;
            color: #1976d2;
            text-decoration: none;
        }
    </style>
</head>
<body>
    <h1>Error</h1>
    <div class="error">{{.}}</div>
    <a href="/">← Back to Upload</a>
</body>
</html>
`
	t, err := template.New("error").Parse(tmpl)
	if err != nil {
		log.Printf("Template parse error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	t.Execute(w, errorMsg)
}

func renderResults(w http.ResponseWriter, status SystemStatus, result InferenceResult) {
	// Convert confidence to percentage (0-100 range) for display
	for i := range result.Detections {
		result.Detections[i].Confidence = result.Detections[i].Confidence * 100
	}

	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Results - YOLO Inference</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        h1 {
            color: #333;
        }
        .results {
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .detection {
            padding: 15px;
            margin: 10px 0;
            background-color: #e8f5e9;
            border-left: 4px solid #4CAF50;
            border-radius: 4px;
        }
        .error {
            color: #d32f2f;
            background-color: #ffebee;
            padding: 15px;
            border-radius: 4px;
            border-left: 4px solid #d32f2f;
        }
        .summary {
            font-size: 18px;
            margin-bottom: 20px;
            padding: 15px;
            background-color: #e3f2fd;
            border-radius: 4px;
        }
        a {
            display: inline-block;
            margin-top: 20px;
            padding: 10px 20px;
            background-color: #4CAF50;
            color: white;
            text-decoration: none;
            border-radius: 4px;
        }
        a:hover {
            background-color: #45a049;
        }
        .class-name {
            font-weight: bold;
            color: #1976d2;
            font-size: 18px;
        }
        .confidence {
            color: #666;
            font-size: 14px;
        }
        .status-bar {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            padding: 20px 30px;
            border-radius: 8px;
            box-shadow: 0 4px 12px rgba(102, 126, 234, 0.3);
            margin-bottom: 20px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .status-item {
            display: flex;
            align-items: center;
            gap: 12px;
        }
        .status-indicator {
            width: 14px;
            height: 14px;
            border-radius: 50%;
            display: inline-block;
            box-shadow: 0 0 8px rgba(255,255,255,0.5);
        }
        .status-indicator.online {
            background-color: #4CAF50;
            animation: pulse-green 2s infinite;
        }
        .status-indicator.offline {
            background-color: #f44336;
            animation: pulse-red 2s infinite;
        }
        .status-indicator.unknown {
            background-color: #ff9800;
            animation: pulse-orange 2s infinite;
        }
        @keyframes pulse-green {
            0%, 100% { box-shadow: 0 0 8px rgba(76, 175, 80, 0.4); }
            50% { box-shadow: 0 0 16px rgba(76, 175, 80, 0.8); }
        }
        @keyframes pulse-red {
            0%, 100% { box-shadow: 0 0 8px rgba(244, 67, 54, 0.4); }
            50% { box-shadow: 0 0 16px rgba(244, 67, 54, 0.8); }
        }
        @keyframes pulse-orange {
            0%, 100% { box-shadow: 0 0 8px rgba(255, 152, 0, 0.4); }
            50% { box-shadow: 0 0 16px rgba(255, 152, 0, 0.8); }
        }
        .status-label {
            font-weight: bold;
            font-size: 15px;
            color: white;
            text-shadow: 0 1px 2px rgba(0,0,0,0.2);
        }
        .training-status {
            font-size: 14px;
            color: rgba(255,255,255,0.95);
            font-weight: 500;
        }
        .spinner-overlay {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0,0,0,0.7);
            z-index: 9999;
            justify-content: center;
            align-items: center;
            flex-direction: column;
        }
        .spinner-overlay.active {
            display: flex;
        }
        .spinner {
            border: 6px solid #f3f3f3;
            border-top: 6px solid #667eea;
            border-radius: 50%;
            width: 60px;
            height: 60px;
            animation: spin 1s linear infinite;
        }
        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
        .spinner-text {
            color: white;
            margin-top: 20px;
            font-size: 18px;
            font-weight: bold;
        }
        .manual-train-btn {
            background-color: #9e9e9e;
            color: white;
            padding: 10px 20px;
            border: none;
            border-radius: 4px;
            cursor: not-allowed;
            font-size: 14px;
            margin-top: 20px;
            opacity: 0.6;
        }
    </style>
</head>
<body>
    <h1>Detection Results</h1>
    <div class="status-bar">
        <div class="status-item">
            <span class="status-indicator {{.Status.NetworkStatus}}"></span>
            <span class="status-label">Network: {{.Status.NetworkStatus}}</span>
        </div>
        <div class="status-item">
            <span class="training-status">Training: {{if .Status.TrainingEnabled}}✓ Enabled{{else}}✗ Disabled{{end}}</span>
        </div>
    </div>
    <div class="results">
        {{if .Result.Error}}
            <div class="error">{{.Result.Error}}</div>
        {{else}}
            <div class="summary">
                <strong>Image:</strong> {{.Result.Image}}<br>
                <strong>Detections Found:</strong> {{.Result.Count}}
            </div>
            {{if gt .Result.Count 0}}
                {{range .Result.Detections}}
                <div class="detection">
                    <div class="class-name">{{.ClassName}}</div>
                    <div class="confidence">Confidence: {{printf "%.1f" .Confidence}}%</div>
                    <div style="font-size: 12px; color: #999; margin-top: 5px;">
                        Class ID: {{.ClassID}} |
                        BBox: ({{printf "%.0f" .BBox.X1}}, {{printf "%.0f" .BBox.Y1}}) to ({{printf "%.0f" .BBox.X2}}, {{printf "%.0f" .BBox.Y2}})
                    </div>
                </div>
                {{end}}
            {{else}}
                <p>No objects detected in the image.</p>
            {{end}}
        {{end}}
    </div>
    <a href="/">← Upload Another Image</a>
</body>
</html>
`
	t, err := template.New("results").Parse(tmpl)
	if err != nil {
		log.Printf("Template parse error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := ResultPageData{
		Status: status,
		Result: result,
	}

	err = t.Execute(w, data)
	if err != nil {
		log.Printf("Template execution error: %v", err)
	}
}
