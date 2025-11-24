// IDAES Web Interface JavaScript

class IDaesAPI {
  constructor(baseURL = '/api/v1') {
    this.baseURL = baseURL;
  }

  async uploadDocument(file) {
    const formData = new FormData();
    formData.append('file', file);

    const response = await fetch(`${this.baseURL}/documents/upload`, {
      method: 'POST',
      body: formData
    });

    return await response.json();
  }

  async analyzeDocument(documentId) {
    const response = await fetch(`${this.baseURL}/documents/analyze`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ document_id: documentId })
    });

    return await response.json();
  }

  async analyzeContent(content, filename = 'untitled.txt') {
    const response = await fetch(`${this.baseURL}/documents/analyze`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({ content, filename })
    });

    return await response.json();
  }

  async getSystemStatus() {
    const response = await fetch(`${this.baseURL}/status`);
    return await response.json();
  }

  async getHealthCheck() {
    const response = await fetch(`${this.baseURL}/health`);
    return await response.json();
  }

  async getMetrics() {
    const response = await fetch(`${this.baseURL}/metrics`);
    return await response.json();
  }
}

// Initialize API client
const api = new IDaesAPI();

// File upload handling
function setupFileUpload() {
  const uploadArea = document.getElementById('upload-area');
  const fileInput = document.getElementById('file-input');
  const progressBar = document.getElementById('progress-bar');
  const progressContainer = document.getElementById('progress-container');
  const resultsContainer = document.getElementById('results-container');

  if (!uploadArea || !fileInput) return;

  // Drag and drop handlers
  uploadArea.addEventListener('dragover', (e) => {
    e.preventDefault();
    uploadArea.classList.add('dragover');
  });

  uploadArea.addEventListener('dragleave', () => {
    uploadArea.classList.remove('dragover');
  });

  uploadArea.addEventListener('drop', (e) => {
    e.preventDefault();
    uploadArea.classList.remove('dragover');

    const files = e.dataTransfer.files;
    if (files.length > 0) {
      handleFileUpload(files[0]);
    }
  });

  // Click to select file
  uploadArea.addEventListener('click', () => {
    fileInput.click();
  });

  fileInput.addEventListener('change', (e) => {
    if (e.target.files.length > 0) {
      handleFileUpload(e.target.files[0]);
    }
  });

  async function handleFileUpload(file) {
    try {
      // Show progress
      if (progressContainer) {
        progressContainer.style.display = 'block';
        progressBar.style.width = '0%';
      }

      // Simulate upload progress
      const progressInterval = setInterval(() => {
        const currentWidth = parseFloat(progressBar.style.width) || 0;
        if (currentWidth < 90) {
          progressBar.style.width = (currentWidth + 10) + '%';
        }
      }, 200);

      // Upload file
      const uploadResult = await api.uploadDocument(file);

      if (!uploadResult.success) {
        throw new Error(uploadResult.error || 'Upload failed');
      }

      // Complete upload progress
      progressBar.style.width = '100%';
      clearInterval(progressInterval);

      // Analyze document
      const analysisResult = await api.analyzeDocument(uploadResult.data.document_id);

      if (!analysisResult.success) {
        throw new Error(analysisResult.error || 'Analysis failed');
      }

      // Display results
      displayAnalysisResults(analysisResult.data);

    } catch (error) {
      console.error('Upload/Analysis error:', error);
      showError('Error: ' + error.message);
    } finally {
      if (progressContainer) {
        setTimeout(() => {
          progressContainer.style.display = 'none';
        }, 1000);
      }
    }
  }
}

// Display analysis results
function displayAnalysisResults(data) {
  const resultsContainer = document.getElementById('results-container');
  if (!resultsContainer) return;

  resultsContainer.innerHTML = `
        <div class="card">
            <h2>Analysis Results</h2>
            <p><strong>Analysis ID:</strong> ${data.analysis_id}</p>
            <p><strong>Processing Time:</strong> ${data.processing_time}</p>
            
            ${displayEntities(data.entities)}
            ${displayCitations(data.citations)}
            ${displayTopics(data.topics)}
            ${displayExtractorsUsed(data.extractors_used)}
        </div>
    `;

  resultsContainer.style.display = 'block';
}

function displayEntities(entities) {
  if (!entities || entities.length === 0) {
    return '<div class="result-section"><h3>Entities</h3><p>No entities found.</p></div>';
  }

  const entitiesHtml = entities.map(entity =>
    `<span class="entity-tag ${entity.type.toLowerCase()}">
            ${entity.text}
            <span class="confidence">${(entity.confidence * 100).toFixed(1)}%</span>
        </span>`
  ).join('');

  return `
        <div class="result-section">
            <h3>Entities (${entities.length})</h3>
            <div>${entitiesHtml}</div>
        </div>
    `;
}

function displayCitations(citations) {
  if (!citations || citations.length === 0) {
    return '<div class="result-section"><h3>Citations</h3><p>No citations found.</p></div>';
  }

  const citationsHtml = citations.map(citation =>
    `<div class="citation-item">
            <strong>${citation.text}</strong>
            <span class="confidence">(${(citation.confidence * 100).toFixed(1)}% confidence, ${citation.format})</span>
        </div>`
  ).join('');

  return `
        <div class="result-section">
            <h3>Citations (${citations.length})</h3>
            <div>${citationsHtml}</div>
        </div>
    `;
}

function displayTopics(topics) {
  if (!topics || topics.length === 0) {
    return '<div class="result-section"><h3>Topics</h3><p>No topics found.</p></div>';
  }

  const topicsHtml = topics.map(topic =>
    `<div class="topic-item">
            <strong>${topic.name}</strong> (Weight: ${topic.weight.toFixed(3)})
            <br><small>Keywords: ${topic.keywords.join(', ')}</small>
        </div>`
  ).join('');

  return `
        <div class="result-section">
            <h3>Topics (${topics.length})</h3>
            <div>${topicsHtml}</div>
        </div>
    `;
}

function displayExtractorsUsed(extractors) {
  if (!extractors || Object.keys(extractors).length === 0) {
    return '';
  }

  const extractorsHtml = Object.entries(extractors).map(([type, method]) =>
    `<span class="entity-tag">${type}: ${method}</span>`
  ).join('');

  return `
        <div class="result-section">
            <h3>Extractors Used</h3>
            <div>${extractorsHtml}</div>
        </div>
    `;
}

// Error handling
function showError(message) {
  const errorContainer = document.getElementById('error-container');
  if (errorContainer) {
    errorContainer.innerHTML = `
            <div class="alert alert-danger">
                <strong>Error:</strong> ${message}
            </div>
        `;
    errorContainer.style.display = 'block';

    setTimeout(() => {
      errorContainer.style.display = 'none';
    }, 5000);
  } else {
    alert('Error: ' + message);
  }
}

// System status monitoring
async function updateSystemStatus() {
  try {
    const health = await api.getHealthCheck();
    const statusElement = document.getElementById('system-status');

    if (statusElement) {
      const statusClass = health.data.status === 'healthy' ? 'healthy' :
        health.data.status === 'degraded' ? 'degraded' : 'unhealthy';

      statusElement.innerHTML = `<span class="status ${statusClass}">${health.data.status}</span>`;
    }
  } catch (error) {
    console.error('Failed to update system status:', error);
  }
}

// Metrics dashboard
async function updateMetrics() {
  try {
    const metrics = await api.getMetrics();

    if (metrics.success) {
      updateMetricCard('total-analyses', metrics.data.pipeline?.total_analyses || 0);
      updateMetricCard('success-rate',
        ((metrics.data.pipeline?.successful_analyses || 0) /
          Math.max(metrics.data.pipeline?.total_analyses || 1, 1) * 100).toFixed(1) + '%');
      updateMetricCard('active-workers',
        metrics.data.workers?.filter(w => w.is_active).length || 0);
      updateMetricCard('queue-depth', metrics.data.orchestrator?.queue_depth || 0);
    }
  } catch (error) {
    console.error('Failed to update metrics:', error);
  }
}

function updateMetricCard(id, value) {
  const element = document.getElementById(id);
  if (element) {
    element.textContent = value;
  }
}

// Content analysis (for direct text input)
function setupContentAnalysis() {
  const analyzeBtn = document.getElementById('analyze-content-btn');
  const contentInput = document.getElementById('content-input');

  if (analyzeBtn && contentInput) {
    analyzeBtn.addEventListener('click', async () => {
      const content = contentInput.value.trim();
      if (!content) {
        showError('Please enter some content to analyze');
        return;
      }

      try {
        analyzeBtn.disabled = true;
        analyzeBtn.textContent = 'Analyzing...';

        const result = await api.analyzeContent(content);

        if (result.success) {
          displayAnalysisResults(result.data);
        } else {
          showError(result.error || 'Analysis failed');
        }
      } catch (error) {
        showError('Analysis error: ' + error.message);
      } finally {
        analyzeBtn.disabled = false;
        analyzeBtn.textContent = 'Analyze Content';
      }
    });
  }
}

// Initialize when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
  setupFileUpload();
  setupContentAnalysis();

  // Update status periodically if on dashboard
  if (document.getElementById('system-status')) {
    updateSystemStatus();
    setInterval(updateSystemStatus, 30000); // Every 30 seconds
  }

  // Update metrics periodically if on dashboard
  if (document.getElementById('total-analyses')) {
    updateMetrics();
    setInterval(updateMetrics, 10000); // Every 10 seconds
  }
});

// Utility functions
function formatBytes(bytes) {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function formatDuration(ms) {
  if (ms < 1000) return ms + 'ms';
  if (ms < 60000) return (ms / 1000).toFixed(1) + 's';
  return (ms / 60000).toFixed(1) + 'm';
}