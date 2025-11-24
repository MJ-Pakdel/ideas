# Test Documents for Entity Extraction

This directory contains test documents covering various document types and edge cases for comprehensive testing of the entity extraction system.

## Document Categories

### 1. Research Papers (`research_papers/`)
- Academic papers with citations, author names, institutions
- Technical content with specialized terminology
- Various formatting styles (LaTeX-like, Word-like, etc.)

### 2. Business Documents (`business_docs/`)
- Corporate reports with financial data and metrics
- Meeting minutes with attendee names and action items
- Presentations with company names and technologies

### 3. Personal Documents (`personal_docs/`)
- Personal letters with names, dates, and locations
- Journal entries with personal reflections
- Travel logs with geographic information

### 4. Technical Documentation (`technical_docs/`)
- Software documentation with technology names
- API documentation with technical specifications
- User manuals with product information

### 5. Legal Documents (`legal_docs/`)
- Contracts with party names and legal entities
- Court documents with case references
- Legal briefs with citations and precedents

### 6. Edge Cases (`edge_cases/`)
- Documents that stress-test the extraction system
- Malformed content, unusual formatting
- Multi-language content, special characters

## Usage

Each test document includes:
- Original content file (`.txt`)
- Expected entities file (`.json`)
- Metadata file with document properties (`.meta.json`)

Test files are used by the comprehensive test suite in `../tests/`.