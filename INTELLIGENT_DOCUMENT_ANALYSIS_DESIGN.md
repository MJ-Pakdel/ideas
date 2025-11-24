# Intelligent Document Analysis Edge System (IDAES)

## Executive Summary

The Intelligent Document Analysis Edge System (IDAES) is a novel enhancement to the fs-vectorize platform that transforms a simple USB document scanner into an intelligent, adaptive document analysis engine. The system performs real-time document classification, content extraction, relationship mapping, and cross-collection intelligence analysis entirely on edge hardware (Raspberry Pi 5).

## Novel Innovation Areas

### Core Novelty
- **Adaptive Edge Intelligence**: Content-aware processing pipeline that adapts vectorization strategies based on document type detection
- **Multi-Scale Relationship Discovery**: Builds citation networks, entity relationships, and topic clusters across both current USB content and historical ChromaDB collections
- **Temporal Knowledge Evolution**: Tracks how topics, entities, and research areas evolve over time across multiple document ingestion sessions with recency-based weighting
- **Cross-Collection Intelligence**: Performs comparative analysis between new content and existing knowledge base to identify patterns, gaps, and evolution
- **Recency-Weighted Intelligence**: Prioritizes recent information in analysis, recommendations, and trend detection while preserving valuable historical context
- **Interactive Edge Interface**: 320x240 TFT display with 4-button navigation providing real-time analysis visualization
- **Contextual Hardware Integration**: Hardware interface adapts to analysis mode, providing relevant controls and information

## System Architecture

### High-Level Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   USB Device    │───▶│  Document        │───▶│  Intelligence   │
│   Detection     │    │  Classification  │    │  Pipeline       │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   ChromaDB      │◀───│  Dual-Model      │◀───│  Cross-Collection│
│   Storage       │    │  Processing      │    │  Analysis       │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│ 320x240 TFT     │◀───│  Relationship    │◀───│  Knowledge      │
│ 4-Button UI     │    │  Mapping         │    │  Evolution      │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   WebUI         │◀───│  Real-time       │◀───│  Interactive    │
│   Dashboard     │    │  Visualization   │    │  Analysis       │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

### Dual-Model Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Dual-Model Engine                        │
├─────────────────────────────┬───────────────────────────────┤
│     Intelligence Model      │      Embedding Model          │
│     (llama3.2:1b)          │   (embeddinggemma:latest)    │
├─────────────────────────────┼───────────────────────────────┤
│ • Document Classification   │ • Semantic Vectorization     │
│ • Entity Extraction         │ • Similarity Embeddings      │
│ • Citation Parsing          │ • Chunk Embeddings           │
│ • Relationship Detection    │ • Query Embeddings           │
│ • Structure Analysis        │ • Cross-lingual Support      │
└─────────────────────────────┴───────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              Unified Processing Pipeline                    │
├─────────────────────────────────────────────────────────────┤
│ 1. Intelligence Model → Document Analysis & Classification  │
│ 2. Adaptive Chunking → Based on Document Type              │
│ 3. Embedding Model → Generate Semantic Vectors             │
│ 4. Relationship Enrichment → Combine Intelligence + Vectors│
│ 5. ChromaDB Storage → Hybrid Storage Strategy              │
└─────────────────────────────────────────────────────────────┘
```

### Component Architecture

#### 1. Document Classification Engine
```
Input Document
     │
     ▼
┌─────────────────┐
│ File Type       │ ──▶ PDF, DOCX, TXT, MD
│ Detection       │
└─────────────────┘
     │
     ▼
┌─────────────────┐
│ Content Type    │ ──▶ Research Paper, Business Doc,
│ Classification  │     Personal Document, Report
└─────────────────┘
     │
     ▼
┌─────────────────┐
│ Structure       │ ──▶ Abstract, Sections, References,
│ Analysis        │     Metadata, Key Entities
└─────────────────┘
```

#### 2. Adaptive Processing Pipeline
```
Document Type
     │
     ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Research Paper  │    │ Business Doc    │    │ Personal Doc    │
│ Processor       │    │ Processor       │    │ Processor       │
└─────────────────┘    └─────────────────┘    └─────────────────┘
     │                          │                          │
     ▼                          ▼                          ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Citation        │    │ Action Items    │    │ Key Topics      │
│ Extraction      │    │ Key Decisions   │    │ Important Dates │
│ Author Analysis │    │ Stakeholders    │    │ Entities        │
│ Methodology     │    │ Metrics         │    │ Categories      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

#### 3. Intelligence Layer Architecture
```
┌─────────────────────────────────────────────────────────────┐
│                    Intelligence Layer                       │
├─────────────────┬─────────────────┬─────────────────────────┤
│ Citation        │ Entity          │ Topic Clustering        │
│ Network Engine  │ Relationship    │ Engine                  │
│                 │ Mapper          │                         │
├─────────────────┼─────────────────┼─────────────────────────┤
│ • Extract refs  │ • People        │ • USB-level clusters   │
│ • Build graphs  │ • Organizations │ • Collection clusters   │
│ • Find clusters │ • Locations     │ • Cross-collection      │
│ • Track lineage │ • Co-occurrence │ • Temporal evolution    │
└─────────────────┴─────────────────┴─────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              Cross-Collection Analysis Engine               │
├─────────────────────────────────────────────────────────────┤
│ • Compare new content vs historical collections             │
│ • Identify topic evolution and drift                        │
│ • Detect research gaps and opportunities                    │
│ • Build temporal knowledge maps with recency weighting     │
│ • Generate recency-aware relationship recommendations       │
│ • Apply time-decay functions for relevance scoring         │
└─────────────────────────────────────────────────────────────┘
```

## Detailed Component Design

### 1. Document Classification Engine

#### Purpose
Intelligently categorize documents to enable adaptive processing strategies.

#### Components

**File Type Detector**
- MIME type analysis
- Content signature detection
- Extension validation

**Content Type Classifier**
- Machine learning model for document categorization
- Features: text patterns, structure, vocabulary
- Categories: Research Paper, Business Document, Personal Document, Report, Legal Document

**Structure Analyzer**
- Research Papers: Abstract, Introduction, Methodology, Results, Discussion, References
- Business Documents: Executive Summary, Objectives, Analysis, Recommendations
- Personal Documents: Free-form content with entity extraction

#### Implementation Strategy
```go
type DocumentClassifier struct {
    FileTypeDetector    *FileTypeDetector
    ContentClassifier   *MLContentClassifier
    StructureAnalyzer   *StructureAnalyzer
}

type DocumentType struct {
    FileType     string
    ContentType  string
    Structure    DocumentStructure
    Confidence   float64
}
```

### 2. Dual-Model Processing Engine

#### Purpose
Leverage specialized models for intelligence processing and semantic vectorization to achieve optimal document analysis and representation.

#### Model Responsibilities

**Intelligence Model (llama3.2:1b)**
- Document type classification
- Entity extraction (people, organizations, locations)
- Citation parsing and relationship detection
- Structure analysis and metadata extraction
- Content summarization and key point identification

**Embedding Model (embeddinggemma:latest)**
- Semantic vectorization of document chunks
- Query embedding generation
- Similarity computation
- Cross-lingual embedding support
- Optimized vector representations

#### Processing Strategies by Document Type

**Research Papers**
- Intelligence Model: Extract citations, authors, methodology, research topics
- Embedding Model: Section-aware chunking with citation context preservation
- Hybrid Storage: Relationship graphs + semantic vectors

**Business Documents**
- Intelligence Model: Identify stakeholders, action items, decisions, metrics
- Embedding Model: Executive summary emphasis with timeline awareness
- Hybrid Storage: Organizational relationships + content vectors

**Personal Documents**
- Intelligence Model: Extract entities, dates, categories, key topics
- Embedding Model: Topic-based chunking with entity-centric organization
- Hybrid Storage: Personal knowledge graphs + semantic content

#### Implementation
```go
type DualModelProcessor struct {
    IntelligenceClient  *ollama.Client  // llama3.2:1b
    EmbeddingClient     *ollama.Client  // embeddinggemma:latest
    ChromaClient        *chroma.Client
    Config              *ModelConfig
}

type ModelConfig struct {
    IntelligenceModel   string // "llama3.2:1b"
    EmbeddingModel      string // "embeddinggemma:latest"
    OllamaURL          string // "http://localhost:11434"
    MaxTokens          int    // 2048
    ConcurrentModels   bool   // true
}

type ProcessingPipeline struct {
    DocumentClassifier  *IntelligenceProcessor
    EntityExtractor     *IntelligenceProcessor
    CitationParser      *IntelligenceProcessor
    AdaptiveChunker     *ChunkingEngine
    SemanticVectorizer  *EmbeddingProcessor
    RelationshipMapper  *RelationshipEngine
}
```

### 3. Intelligence Processing Components

#### Document Classification Engine
**Purpose**: Intelligently categorize documents using llama3.2:1b for adaptive processing.

**Implementation**:
```go
type DocumentClassifier struct {
    IntelligenceModel *ollama.Client
}

func (dc *DocumentClassifier) ClassifyDocument(content string) DocumentType {
    prompt := fmt.Sprintf(`Analyze this document and classify it:

Content: %s

Classify as one of: research_paper, business_document, personal_document, report, legal_document
Provide confidence score and key indicators.`, content[:1000])
    
    response := dc.IntelligenceModel.Generate("llama3.2:1b", prompt)
    return parseDocumentType(response)
}
```

#### Entity Extraction Engine
**Purpose**: Extract and categorize entities using llama3.2:1b for relationship mapping.

**Implementation**:
```go
type EntityExtractor struct {
    IntelligenceModel *ollama.Client
}

func (ee *EntityExtractor) ExtractEntities(content string, docType DocumentType) []Entity {
    prompt := fmt.Sprintf(`Extract entities from this %s:

Content: %s

Extract:
- People (names, roles)
- Organizations (companies, institutions)
- Locations (cities, countries)
- Dates and times
- Key concepts/topics

Format as JSON.`, docType.ContentType, content)
    
    response := ee.IntelligenceModel.Generate("llama3.2:1b", prompt)
    return parseEntities(response)
}
```

#### Citation Network Engine
**Purpose**: Extract and analyze citation relationships using llama3.2:1b for research knowledge graphs.

**Features**:
- Multiple citation format support (APA, MLA, Chicago, IEEE)
- DOI and URL extraction using pattern matching
- Author disambiguation via llama3.2:1b
- Publication venue identification
- Cross-collection citation matching

**Implementation**:
```go
type CitationExtractor struct {
    IntelligenceModel *ollama.Client
}

func (ce *CitationExtractor) ExtractCitations(content string) []Citation {
    prompt := fmt.Sprintf(`Extract all citations from this academic text:

Content: %s

For each citation, extract:
- Authors
- Title
- Publication venue
- Year
- DOI (if present)
- Citation format used

Format as structured JSON.`, content)
    
    response := ce.IntelligenceModel.Generate("llama3.2:1b", prompt)
    return parseCitations(response)
}

type Citation struct {
    ID          string
    Authors     []Author
    Title       string
    Venue       string
    Year        int
    DOI         string
    URL         string
    Format      string // APA, MLA, Chicago, IEEE
    CitingDocs  []string
    Confidence  float64
}

type Author struct {
    Name        string
    Affiliation string
    ORCID       string
}

type CitationNetwork struct {
    Citations   map[string]*Citation
    Edges       []CitationEdge
    Clusters    []CitationCluster
    Lineages    []ResearchLineage
}
```

### 4. Recency Weighting Engine

#### Purpose
Apply temporal relevance scoring to prioritize recent information while maintaining valuable historical context.

#### Features

**Time-Decay Functions**
- Exponential decay for research papers (half-life: 2-5 years)
- Linear decay for business documents (half-life: 6-12 months)
- Custom decay curves for different document types
- Configurable decay parameters per domain

**Recency Scoring**
- Document-level recency based on creation/modification dates
- Entity-level recency based on first/last mention
- Topic-level recency based on emergence and activity
- Citation-level recency for research lineage tracking

**Weighted Analysis**
- Recency-weighted similarity calculations
- Time-biased recommendation generation
- Temporal trend detection with recent emphasis
- Evolution tracking with recency acceleration

#### Implementation
```go
type RecencyWeightingEngine struct {
    DecayFunctions  map[DocumentType]*DecayFunction
    TimeHorizons    map[AnalysisType]time.Duration
    WeightingRules  []WeightingRule
}

type RecencyScore struct {
    BaseScore      float64
    RecencyWeight  float64
    FinalScore     float64
    DecayFunction  string
    Timestamp      time.Time
}

type DecayFunction struct {
    Type       DecayType  // Exponential, Linear, Logarithmic
    HalfLife   time.Duration
    MinWeight  float64    // Minimum weight (never fully decay)
    MaxAge     time.Duration
}
```

### 5. Entity Relationship Mapper

#### Purpose
Identify and track relationships between people, organizations, and concepts across documents.

#### Entity Types
- **People**: Authors, researchers, executives, contacts
- **Organizations**: Universities, companies, institutions
- **Locations**: Geographic references, research sites
- **Concepts**: Technical terms, methodologies, topics
- **Temporal**: Dates, events, timelines

#### Relationship Analysis
- Co-occurrence analysis
- Temporal relationship tracking
- Hierarchical relationship detection
- Cross-document entity resolution

### 6. Cross-Collection Intelligence Engine

#### Purpose
Analyze relationships and evolution across multiple ChromaDB collections to identify patterns, gaps, and opportunities.

#### Features
- **Collection Comparison**: Compare new USB content with existing collections
- **Topic Evolution**: Track how research areas develop over time
- **Knowledge Gaps**: Identify missing connections and research opportunities
- **Trend Analysis**: Detect emerging topics and declining areas
- **Recommendation Engine**: Suggest related documents and research directions

#### Implementation
```go
type CrossCollectionAnalyzer struct {
    ChromaClient     *chroma.Client
    Collections      []string
    EvolutionTracker *EvolutionTracker
    RecencyWeighter  *RecencyWeightingEngine
    GapAnalyzer      *GapAnalyzer
}

type KnowledgeInsight struct {
    Type           InsightType
    Description    string
    Evidence       []string
    Confidence     float64
    RecencyScore   float64
    Actionable     []Recommendation
}
```

#### Implementation
```go
type Entity struct {
    ID          string
    Type        EntityType
    Name        string
    Aliases     []string
    Context     []string
    Documents   []string
    FirstSeen   time.Time
    LastSeen    time.Time
}

type EntityRelationship struct {
    Source      string
    Target      string
    Type        RelationType
    Strength    float64
    Evidence    []string
    Documents   []string
}
```

## Hardware Integration

### Enhanced Display System (320x240 TFT)

#### Display Modes
- **System Status**: ChromaDB/Ollama health, USB port status
- **Document Analysis**: Current processing, document type, extracted data
- **Relationship Map**: Entity networks, citation graphs
- **Knowledge Evolution**: Topic trends, discovery timeline
- **WebUI Status**: Interface access, connection info

#### Visual Design
- Landscape orientation (320x240)
- Multi-panel layouts
- Real-time status indicators
- Context-aware information display

### 4-Button Navigation Interface

#### Button Layout
```
GPIO 17: Mode Switch    │ GPIO 22: Action/Select
GPIO 23: Nav Up/Prev    │ GPIO 27: Nav Down/Next
```

#### Context-Sensitive Functions

**System Status Mode:**
- Button 17: Switch to Document Analysis
- Button 22: Trigger manual USB scan
- Button 23: Show IP address (3s display)
- Button 27: Clear ChromaDB collections

**Document Analysis Mode:**
- Button 17: Switch to Relationship Map
- Button 22: Refresh current analysis
- Button 23: Previous document in queue
- Button 27: Next document in queue

**Relationship Map Mode:**
- Button 17: Switch to Knowledge Evolution
- Button 22: Focus on selected entity
- Button 23: Previous entity/relationship
- Button 27: Next entity/relationship

**Knowledge Evolution Mode:**
- Button 17: Switch to WebUI Status
- Button 22: Show detailed trends
- Button 23: Previous time period
- Button 27: Next time period

**WebUI Status Mode:**
- Button 17: Switch to System Status
- Button 22: Rotate display orientation
- Button 23: Scroll up through status
- Button 27: Scroll down through status

### Enhanced WebUI Dashboard

#### IDAES-Specific Endpoints
```
/idaes                          # Main IDAES dashboard
/idaes/documents               # Document analysis interface
/idaes/relationships           # Relationship visualization
/idaes/knowledge              # Knowledge evolution charts

/api/idaes/documents          # Document analysis API
/api/idaes/relationships      # Relationship data API
/api/idaes/knowledge/trends   # Knowledge evolution API
/api/idaes/knowledge/gaps     # Knowledge gap analysis
```

#### Real-time Features
- WebSocket updates for live analysis
- Interactive relationship graphs
- Temporal knowledge visualization
- Cross-collection insights

## Service Architecture

### Single-Service Design

IDaES runs as **one Linux systemd service** with internal service modules communicating via Go channels and shared memory.

```
systemd service: fs-vectorize.service
├── Main Process (orchestrator)
├── USB Service (goroutine)
├── Intelligence Service (goroutine) 
├── Display Service (goroutine)
├── WebUI Service (goroutine)
└── Storage Service (goroutine)
```

### Service Boundaries

**Orchestrator Service**
- Service lifecycle management
- Event routing and coordination
- Configuration management
- Health monitoring

**USB Service**
- Device detection and mounting
- File system scanning
- Device lifecycle events

**Intelligence Service**
- Document classification and analysis
- Entity extraction and relationship mapping
- Knowledge graph construction
- Cross-collection analysis

**Display Service**
- Hardware display management
- Multi-mode UI rendering
- Real-time status updates

**WebUI Service**
- HTTP server and WebSocket management
- API endpoints for IDAES features
- Real-time dashboard updates

**Storage Service**
- ChromaDB operations
- Vector database management
- Collection lifecycle

### Event-Driven Communication (Go Concurrency Philosophy)

**"Do not communicate by sharing memory; instead, share memory by communicating."**

IDaES follows Go's concurrency philosophy through immutable events and channel-based communication:

```go
// pkg/events/events.go
type Event interface {
    Type() string
    Timestamp() time.Time
    // Events are immutable - no setters, only getters
}

// Immutable event structures for safe channel passing
type USBDeviceConnected struct {
    devicePath string  // Private fields prevent mutation
    mountPath  string
    fileSystem string
    timestamp  time.Time
}

// Getters provide read-only access
func (e USBDeviceConnected) DevicePath() string { return e.devicePath }
func (e USBDeviceConnected) MountPath() string { return e.mountPath }
func (e USBDeviceConnected) Type() string { return "usb.device.connected" }
func (e USBDeviceConnected) Timestamp() time.Time { return e.timestamp }

type DocumentAnalyzed struct {
    filePath      string
    analysis      DocumentAnalysis  // Immutable copy
    entities      []Entity         // Immutable slice
    relationships []Relationship   // Immutable slice
    timestamp     time.Time
}

type KnowledgeGraphUpdated struct {
    newEntities      []Entity
    newRelationships []Relationship
    collections      []string
    timestamp        time.Time
}

// Channel-based service coordination
type ServiceChannels struct {
    Events    chan Event           // Broadcast events
    Commands  chan Command         // Service commands
    Responses chan Response        // Command responses
    Shutdown  chan struct{}        // Graceful shutdown
}
```

### Service Interfaces

```go
// services/interfaces.go
type Service interface {
    Start(ctx context.Context, eventBus chan<- Event) error
    Stop(ctx context.Context) error
    Health() error
}

type IntelligenceService interface {
    Service
    AnalyzeDocument(ctx context.Context, doc Document) (*DocumentAnalysis, error)
    ExtractEntities(ctx context.Context, content string) ([]Entity, error)
    BuildKnowledgeGraph(ctx context.Context, entities []Entity) error
}

type DisplayService interface {
    Service
    SetMode(mode DisplayMode)
    UpdateStatus(status SystemStatus)
    ShowAnalysis(analysis *DocumentAnalysis)
}
```

## Implementation Architecture

### Core Components

```go
// services/orchestrator/orchestrator.go
type Orchestrator struct {
    services    map[string]Service
    eventBus    chan Event
    config      *config.Config
    logger      *slog.Logger
}

// services/intelligence/service.go
type IntelligenceService struct {
    ollamaManager *embeddings.OllamaManager
    chromaManager *vectordb.ChromaManager
    entityStore   *EntityStore
    knowledgeGraph *KnowledgeGraph
}

// services/display/service.go
type DisplayService struct {
    display     *display.PiTFTDisplay
    currentMode DisplayMode
    statusData  *StatusData
    analysisData *AnalysisData
}
```

### Integration Points

1. **Daemon Integration**: Auto-detects IDAES mode and configures 4-button setup
2. **Display Integration**: Extends existing PiTFT with multi-mode support
3. **WebUI Integration**: Adds IDAES routes and real-time updates
4. **Command Line**: `-idaes` flag auto-enables display, GPIO, and WebUI

## Usage Examples

### Basic IDAES Operation
```bash
# Start IDAES mode (auto-enables all features)
./fs-vectorize -daemon -idaes

# Or use Makefile
make run-idaes

# Deploy to Raspberry Pi
make deploy-rpi-idaes RPI_HOST=pi@192.168.1.100
```

### Hardware Interaction
1. **Insert USB device** → Automatic detection and analysis begins
2. **Press Mode button** → Cycle through analysis views on display
3. **Use navigation buttons** → Browse entities, documents, relationships
4. **Access WebUI** → `http://pi.local:8001/idaes` for detailed analysis

### Analysis Workflow
1. **Document Detection** → Classification → Adaptive Processing
2. **Entity Extraction** → Relationship Mapping → Network Analysis
3. **Cross-Collection** → Comparison → Evolution Tracking
4. **Knowledge Gaps** → Recommendations → Insight Generation

## Performance Considerations

### Raspberry Pi 5 Optimization
- **CPU**: 4-core ARM64 - optimal for concurrent analysis
- **Memory**: 8GB RAM - sufficient for knowledge graphs
- **Storage**: Fast SD card/USB 3.0 for ChromaDB
- **Display**: Hardware-accelerated framebuffer rendering

### Scalability
- **Document Processing**: Configurable worker pools
- **Relationship Analysis**: Incremental graph building
- **Knowledge Evolution**: Efficient temporal indexing
- **WebUI**: Async updates with WebSocket streamingases     []string
    Attributes  map[string]interface{}
    Documents   []string
}

type Relationship struct {
    Source      string
    Target      string
    Type        RelationType
    Strength    float64
    Context     []string
    Temporal    TimeRange
}
```

### 7. Multi-Scale Topic Clustering Engine

#### Purpose
Discover and organize topics at multiple scales: document, USB session, collection, and cross-collection levels.

#### Clustering Levels

**USB-Level Clustering**
- Topics within current USB drive content
- Immediate thematic organization
- Real-time cluster formation

**Collection-Level Clustering**
- Topics within individual ChromaDB collections
- Historical thematic analysis
- Collection characterization

**Cross-Collection Clustering**
- Topics spanning multiple collections
- Global thematic landscape
- Knowledge domain mapping

**Temporal Evolution Tracking**
- Topic emergence and decline
- Research trend analysis
- Knowledge evolution patterns

#### Algorithm Design
```go
type TopicCluster struct {
    ID          string
    Level       ClusterLevel
    Topics      []Topic
    Documents   []string
    Centroid    []float64
    Coherence   float64
    Temporal    TimeRange
}

type TopicEvolution struct {
    Topic        string
    Timeline     []TopicSnapshot
    Trend        TrendDirection
    Velocity     float64
    RecencyScore float64
    Predictions  []FutureTopic
}
```

### 8. Cross-Collection Analysis Engine

#### Purpose
Perform intelligent analysis across all ChromaDB collections to identify patterns, evolution, and opportunities.

#### Analysis Types

**Content Comparison**
- New vs existing content similarity
- Duplicate detection and handling
- Content gap identification

**Topic Evolution Analysis**
- Research area development over time
- Emerging vs declining topics
- Interdisciplinary connections

**Knowledge Graph Construction**
- Global entity relationship mapping
- Cross-collection citation networks
- Comprehensive knowledge representation

**Recommendation Engine**
- Related document suggestions
- Research opportunity identification
- Knowledge gap highlighting

#### Implementation
```go
type CrossCollectionAnalyzer struct {
    Collections     []ChromaCollection
    GlobalGraph     *KnowledgeGraph
    EvolutionTracker *TopicEvolutionTracker
    Recommender     *RecommendationEngine
}

type AnalysisResult struct {
    Similarities    []DocumentSimilarity
    Evolution       []TopicEvolution
    Recommendations []Recommendation
    Insights        []KnowledgeInsight
}
```

### 9. Adaptive Summarization Engine

#### Purpose
Generate intelligent, context-aware summaries based on document type and user needs.

#### Summarization Strategies

**Research Papers**
- Key findings extraction
- Methodology summary
- Contribution highlights
- Future work identification

**Business Documents**
- Executive summary generation
- Key decision points
- Action item extraction
- Risk and opportunity identification

**Personal Documents**
- Main topic identification
- Important entity extraction
- Key date and event summary

#### Implementation
```go
type AdaptiveSummarizer struct {
    ResearchSummarizer  *ResearchSummarizer
    BusinessSummarizer  *BusinessSummarizer
    PersonalSummarizer  *PersonalSummarizer
    TemplateEngine      *SummaryTemplateEngine
}

type Summary struct {
    Type        SummaryType
    Content     string
    KeyPoints   []string
    Entities    []Entity
    Confidence  float64
    Metadata    map[string]interface{}
}
```

## Data Flow Architecture

### Processing Pipeline Flow

```
USB Device Insertion
        │
        ▼
┌─────────────────┐
│ File Discovery  │
│ & Enumeration   │
└─────────────────┘
        │
        ▼
┌─────────────────┐
│ Document        │ ──▶ Research Paper │ Business Doc │ Personal Doc
│ Classification  │                    │              │
└─────────────────┘                    │              │
        │                              │              │
        ▼                              ▼              ▼
┌─────────────────┐              ┌─────────────┐ ┌─────────────┐
│ Content         │              │ Specialized │ │ Specialized │
│ Extraction      │              │ Processing  │ │ Processing  │
└─────────────────┘              └─────────────┘ └─────────────┘
        │                              │              │
        ▼                              ▼              ▼
┌─────────────────┐              ┌─────────────────────────────┐
│ Adaptive        │              │     Intelligence Layer      │
│ Vectorization   │              │                             │
└─────────────────┘              │ • Citation Networks         │
        │                        │ • Entity Relationships      │
        ▼                        │ • Topic Clustering          │
┌─────────────────┐              │ • Cross-Collection Analysis │
│ ChromaDB        │              └─────────────────────────────┘
│ Storage         │                              │
└─────────────────┘                              ▼
        │                        ┌─────────────────────────────┐
        ▼                        │     Results Integration     │
┌─────────────────┐              │                             │
│ Cross-Collection│◀─────────────│ • Knowledge Graph Update    │
│ Analysis        │              │ • Relationship Mapping      │
└─────────────────┘              │ • Evolution Tracking        │
        │                        └─────────────────────────────┘
        ▼
┌─────────────────┐
│ Display Update  │
│ & Notification  │
└─────────────────┘
```

### Data Storage Architecture

```
ChromaDB Collections Structure:

┌─────────────────────────────────────────────────────────────┐
│                    Collection Hierarchy                     │
├─────────────────┬─────────────────┬─────────────────────────┤
│ Document        │ Intelligence    │ Cross-Collection        │
│ Collections     │ Collections     │ Analysis                │
├─────────────────┼─────────────────┼─────────────────────────┤
│ • usb_session_  │ • citations     │ • global_topics         │
│   {timestamp}   │ • entities      │ • topic_evolution       │
│ • research_     │ • relationships │ • knowledge_graph       │
│   papers        │ • topics        │ • recommendations       │
│ • business_docs │ • summaries     │ • insights              │
│ • personal_docs │                 │                         │
└─────────────────┴─────────────────┴─────────────────────────┘
```

## Implementation Plan

### Phase 1: Foundation (Weeks 1-2)
- Document classification engine
- Basic adaptive vectorization
- Enhanced file processing pipeline

### Phase 2: Intelligence Layer (Weeks 3-4)
- Citation extraction and network building
- Entity relationship mapping
- Basic topic clustering

### Phase 3: Cross-Collection Analysis (Weeks 5-6)
- Cross-collection comparison engine
- Topic evolution tracking
- Knowledge graph construction

### Phase 4: Advanced Features (Weeks 7-8)
- Adaptive summarization
- Recommendation engine
- Display integration and visualization

### Phase 5: Optimization & Testing (Weeks 9-10)
- Performance optimization for Pi 5
- Comprehensive testing
- Documentation and deployment

## Technical Specifications

### Hardware Requirements
- **Primary**: Raspberry Pi 5 (4GB+ RAM recommended)
- **Storage**: High-speed SD card (Class 10/U3) or USB 3.0 storage
- **Display**: PiTFT 2.4" (240x240) for status and results
- **Input**: GPIO buttons for basic interaction

### Software Dependencies
- **Go**: 1.20+ for main application
- **ChromaDB**: Vector database for document storage
- **ML Libraries**: For document classification and NLP
- **System**: udev, udisks2 for USB handling

### Performance Targets
- **Document Classification**: <500ms per document
- **Vectorization**: <2s per page
- **Cross-Collection Analysis**: <30s for full analysis
- **Memory Usage**: <2GB total system usage

## Novel Aspects Summary

### Primary Innovations

1. **Adaptive Edge Intelligence**
   - Content-aware processing without cloud dependency
   - Real-time document type detection and specialized handling
   - Edge-optimized ML models for classification

2. **Multi-Scale Relationship Discovery**
   - Citation network construction and analysis
   - Entity relationship mapping across documents
   - Topic clustering at multiple organizational levels

3. **Temporal Knowledge Evolution with Recency Weighting**
   - Cross-session knowledge building with time-decay functions
   - Topic evolution tracking with recent emphasis
   - Research trend analysis prioritizing current developments
   - Configurable decay curves for different document domains

4. **Cross-Collection Intelligence**
   - Comparative analysis across historical data with recency bias
   - Knowledge gap identification emphasizing recent gaps
   - Intelligent recommendation generation with temporal relevance

### Competitive Advantages

- **Privacy-First**: All processing happens locally on edge device
- **Adaptive**: Intelligent processing based on content understanding
- **Comprehensive**: Handles multiple document types with specialized strategies
- **Evolutionary**: Builds knowledge over time across multiple sessions
- **Integrated**: Combines document processing with relationship discovery

## Future Enhancement Opportunities

### Advanced Features
- **Multi-language Support**: Extend to non-English documents
- **Image Analysis**: OCR and image content extraction
- **Audio Processing**: Transcription and analysis of audio files
- **Collaborative Features**: Multi-device knowledge sharing

### AI/ML Enhancements
- **Custom Model Training**: Train models on user's specific document corpus
- **Predictive Analytics**: Predict research directions and opportunities
- **Automated Insights**: Generate research insights and hypotheses
- **Natural Language Queries**: Allow natural language search across collections

### Integration Possibilities
- **Research Tools**: Integration with reference managers (Zotero, Mendeley)
- **Productivity Suites**: Export to document creation tools
- **Visualization**: Advanced network and evolution visualizations
- **APIs**: Provide programmatic access to intelligence layer

This design represents a significant advancement in edge-based document intelligence, combining multiple novel approaches to create a comprehensive, adaptive, and intelligent document analysis system.

### 4. Semantic Vectorization Engine

#### Purpose
Generate high-quality semantic embeddings using embeddinggemma:latest for similarity search and retrieval.

**Implementation**:
```go
type SemanticVectorizer struct {
    EmbeddingModel *ollama.Client
}

func (sv *SemanticVectorizer) GenerateEmbeddings(chunks []Chunk) [][]float64 {
    var embeddings [][]float64
    
    for _, chunk := range chunks {
        // Enhance chunk with entity context for better embeddings
        enrichedText := sv.enrichChunkWithContext(chunk)
        
        embedding := sv.EmbeddingModel.Embeddings("embeddinggemma:latest", enrichedText)
        embeddings = append(embeddings, embedding)
    }
    
    return embeddings
}

func (sv *SemanticVectorizer) enrichChunkWithContext(chunk Chunk) string {
    // Add entity and relationship context to improve embedding quality
    context := fmt.Sprintf("Document Type: %s\nEntities: %s\nContent: %s", 
        chunk.DocumentType, 
        strings.Join(chunk.Entities, ", "), 
        chunk.Text)
    return context
}
```

### 5. Hybrid Storage Strategy

#### Purpose
Combine semantic vectors with structured relationship data for comprehensive knowledge representation.

**Storage Collections**:
```go
type StorageManager struct {
    DocumentCollection     *chroma.Collection // Main document chunks with embeddings
    RelationshipCollection *chroma.Collection // Entity relationships and citations
    MetadataCollection     *chroma.Collection // Document metadata and classifications
}

type DocumentChunk struct {
    ID           string
    Text         string
    Embedding    []float64
    DocumentType string
    Entities     []string
    Citations    []string
    Metadata     map[string]interface{}
    Timestamp    time.Time
}

type Relationship struct {
    ID           string
    Type         string // "citation", "collaboration", "mention"
    SourceEntity string
    TargetEntity string
    Context      string
    Strength     float64
    DocumentID   string
    Timestamp    time.Time
}
```

**Benefits**:
- **Semantic Search**: Fast similarity queries via embeddings
- **Structured Queries**: Precise relationship traversal
- **Hybrid Retrieval**: Combine semantic + structural results
- **Temporal Analysis**: Track relationship evolution over time
- **Cross-Collection Intelligence**: Link related content across USB sessions

## Model Configuration and Deployment

### Raspberry Pi 5 Optimization (16GB RAM)

**Enhanced Memory Management**:
```yaml
models:
  intelligence:
    name: "llama3.2:3b"  # Upgraded from 1b with 16GB RAM
    memory_limit: "6GB"
    context_length: 8192  # Increased context window
    
  embedding:
    name: "embeddinggemma:latest"
    memory_limit: "3GB"
    batch_size: 128  # Larger batches for efficiency

  # Optional: Additional specialized models
  citation_model:
    name: "llama3.2:1b"  # Dedicated citation parsing
    memory_limit: "2GB"
    context_length: 4096

system:
  concurrent_models: true
  parallel_processing: true  # Enable parallel document processing
  memory_buffer: "3GB"  # Larger buffer for system operations
  max_concurrent_docs: 4  # Process multiple documents simultaneously
  swap_enabled: false  # Disable swap with sufficient RAM
```

**Enhanced Performance Tuning**:
- **Larger Intelligence Model**: Use llama3.2:3b for better accuracy and reasoning
- **Parallel Processing**: Process multiple documents concurrently
- **Larger Batch Sizes**: Process 128+ chunks simultaneously for embeddings
- **Extended Context**: 8K context window for better document understanding
- **Model Caching**: Keep all models loaded in memory simultaneously
- **Advanced Caching**: Cache entity graphs, citation networks, and document classifications
- **Real-time Analysis**: Enable live cross-collection intelligence during processing

### Command Line Interface

```bash
# Start IDAES daemon with dual models
./fs-vectorize -daemon \
  -intelligence-model llama3.2:1b \
  -embedding-model embeddinggemma:latest \
  -enable-intelligence \
  -workers 2 \
  -enable-display \
  -enable-gpio

# Configuration options
-intelligence-model string    Intelligence model name (default "llama3.2:1b")
-embedding-model string       Embedding model name (default "embeddinggemma:latest")
-enable-intelligence         Enable intelligent document analysis
-ollama-url string           Ollama server URL (default "http://localhost:11434")
-max-tokens int              Maximum tokens for intelligence model (default 2048)
-concurrent-models           Run models concurrently (default true)
```

## Implementation Benefits

### Enhanced Capabilities
- **Specialized Processing**: Each model optimized for its specific task
- **Better Accuracy**: llama3.2:1b excels at classification and extraction
- **Efficient Embeddings**: embeddinggemma optimized for semantic vectors
- **Resource Management**: Concurrent operation on Pi 5's 8GB RAM
- **Scalable Architecture**: Easy to swap or upgrade individual models

### Improved User Experience
- **Transparent Operation**: Standard RAG queries work without changes
- **Rich Metadata**: Enhanced search results with entity and relationship context
- **Cross-Collection Intelligence**: Discover connections across document sets
- **Real-time Analysis**: Live processing feedback via TFT display
- **Adaptive Processing**: Content-aware strategies for different document types