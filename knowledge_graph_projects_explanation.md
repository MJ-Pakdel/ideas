# Knowledge Graph Methodologies by Example

This document contains only executable examples so you can reproduce the methodology without the original source code.

---

## 1. Auto-Graph Example: User Authentication Repo

### Inputs
- `user.py` (Class User, login method)
- `auth.py` (Class AuthService, validate method)
- `README.md` (Text "User Management", "Authentication")

### Step-by-Step Logic
1. **Scan**: Find all `*.py` and `*.md` files.
2. **Extract**: Run LLM on each file *individually*.
   - Prompt: "Extract modules, classes, functions, and how they relate."
3. **Construct Nodes**:
   - `user.py` → Node(Module)
   - `class User` → Node(Class)
   - `def login` → Node(Function)
4. **Construct Edges** (The Graph):
   - `(user.py)-[:CONTAINS]->(User)`
   - `(User)-[:CONTAINS]->(login)`
   - `(AuthService)-[:USES]->(User)`
5. **Deduplicate**:
   - If `README.md` mentions "User Class" and `user.py` defines "User Class":
   - Calculate vector similarity of names. If > 0.85, merge them into one node.
6. **Query**:
   - "How does auth work?" → Graph lookup finds `AuthService` node + all connected code (`login`, `User`, `README`).

---

## 2. IDAES Example: Intelligent Document Analysis Edge System

**IDAES** = **I**ntelligent **D**ocument **A**nalysis **E**dge **S**ystem.

### Scenario
You drop 3 files onto a USB stick plugged into a Raspberry Pi:
1. `neural_networks_2023.pdf` (Research Paper)
2. `deep_learning_2024.pdf` (Research Paper)
3. `ml_applications_2024.pdf` (Business Report)

### Part A: Indexing Pipeline (Background Process)
This happens automatically when files are detected. The user is not involved yet.

| Step | Action | Who does it? | Details |
|------|--------|--------------|---------|
| 1 | **Dual-Model Pass** | **LLM #1 + Embedder** | **Supported Types**: Research, Business, Legal, News, Technical.<br>**Input**: First 1k chars.<br>**Output A (LLM)**: "Type: Research". Finds entities: "Dr. Smith", "OpenAI".<br>**Output B (Embedder)**: 384-dim vector (saved for Step 7). |
| 2 | **Routing** | **Python/Go Code** | Reads Output A ("Research"). Sets `mode = research`. |
| 3 | **Load Config** | **Config File** | Loads "Research Checklist" from YAML (e.g., "Find authors, citations"). |
| 4 | **Graph-Aware Chunking** | **Code** | Splits text. **Rules**: <br>1. Break at paragraphs/headers.<br>2. **Never split inside an entity** found in Step 1 (e.g., keep "Dr. Smith" together).<br>3. **Hard Limit**: If entity > 2048 chars, force split (rare edge case). |
| 5 | **Extraction** | **LLM #2** | **Input**: Full text chunks + Checklist.<br>**Output**: JSON with type-specific fields.<br>**Research**: authors, citations, methodology.<br>**Business**: stakeholders, budget, decisions.<br>**Legal**: parties, dates, clauses.<br>**Example**: `{"authors": ["Dr. Smith"], "citations": ["Neural Networks 2023"]}`. |
| 6 | **Storage** | **Database** | Saves chunks + metadata to ChromaDB. |
| 7 | **Cross-Analysis** | **Code + Math** | **Pairwise Similarity**: Computes vector similarity between ALL pairs of existing documents.<br>Example with 3 docs: 3 comparisons (2023↔2024a: 0.87, 2023↔2024b: 0.65, 2024a↔2024b: 0.72).<br>**Citation Links**: Uses Step 5 JSON to create "cites" relationships (e.g., 2024a cites 2023).<br>**Storage**: Saves as database links: `{"source": "deep_learning_2024.pdf", "target": "neural_networks_2023.pdf", "type": "cites", "similarity": 0.87}` and `{"source": "ml_applications_2024.pdf", "target": "neural_networks_2023.pdf", "type": "similar", "similarity": 0.65}`. Used later in retrieval. |
| 8 | **Evolution Tracking** | **Code** | Analyzes temporal patterns in topics.<br>Example: Detects "Neural Networks" (2023) evolved into "Deep Learning" (2024).<br>Also detects cross-domain links: Research topic "Neural Networks" connects to Business topic "ML Applications" via similarity 0.65.<br>Saves: `{"topic": "neural_networks", "evolved_to": "deep_learning", "year_span": "2023-2024"}` and `{"topic": "neural_networks", "connects_to": "ml_applications", "via": "similarity", "strength": 0.65}`. |

### Part B: Retrieval Pipeline (User Asks a Question)
User asks: *"How has neural network research evolved into business applications?"*

| Step | Action | Logic |
|------|--------|-------|
| 1 | **Vector Search** | Regular retrieval: Finds `neural_networks_2023.pdf` directly (high similarity to "neural network"). |
| 2 | **Cross-Collection Search** | Uses pre-computed **Cross-Analysis (Step 7)** links to find related docs:<br>- **Citation link**: `deep_learning_2024.pdf` (cites the 2023 paper, similarity 0.87).<br>- **Similarity link**: `ml_applications_2024.pdf` (high similarity 0.65 to 2023 paper). |
| 3 | **Base Scoring** | Each found document gets its **base score** from Step 7 similarity: `deep_learning_2024.pdf` = 0.87, `ml_applications_2024.pdf` = 0.65. |
| 4 | **Time Decay Scoring** | Applies **Config (Step 3)** Half-Life to final ranking:<br>- `2023 Paper`: Base score * 0.7 (older research paper, half-life 730 days).<br>- `2024 Deep Learning`: 0.87 * 1.0 (recent research, half-life 730 days).<br>- `2024 ML Applications`: 0.65 * 1.0 (recent business, half-life 180 days). |
| 5 | **Answer Generation** | LLM receives ranked results: "Here is the 2023 foundation (score 0.7), the 2024 research update (score 0.87), and the 2024 business case (score 0.65)." |

### The "Adaptive" Config (You write this)
The AI follows this list. It does not guess.

```yaml
document_types:
  research:
    checklist: ["authors", "methodology", "citations"] # Used in Step 5 (Extraction)
    decay_half_life_days: 730  (2 years)               # Used in Part B Step 3 (Scoring)

  business:
    checklist: ["stakeholders", "budget", "risks"]     # Used in Step 5
    decay_half_life_days: 180  (6 months)              # Used in Part B Step 3
```

### Graph-Aware Chunking Details
**Goal**: Preserve context so "OpenAI" isn't split into "Open" and "AI".

1. **Input**: Text stream.
2. **Constraint 1 (Entity)**: "Dr. Smith" (chars 100-109). Do not split here.
3. **Constraint 2 (Semantic)**: Paragraph break at char 150. Good split point.
4. **Constraint 3 (Size)**: Max 2048 chars.
   - *Scenario*: Entity is 3000 chars long (e.g., massive DNA sequence)?
   - *Fallback*: Force split at 2048. We lose entity integrity to save memory/processing.

### Where Metadata is Used (Explicitly)
- **Extracted Authors (Step 5)**: Used in **Retrieval** to filter ("Show papers by Dr. Smith") or link ("Find other papers by this author").
- **Extracted Citations (Step 5)**: Used in **Indexing (Step 7)** to build the hard link between Document A and Document B.
- **Vectors (Step 1)**: Used in **Indexing (Step 7)** to cluster topics and in **Retrieval (Step 1)** to find the starting point.
