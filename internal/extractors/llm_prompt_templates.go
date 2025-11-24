package extractors

// Document classification prompt
func getDocumentClassificationPrompt() string {
	return `Analyze the following document and classify it into one of these categories:
- research_paper: Academic research papers, studies, theses
- business_document: Business reports, memos, proposals, meeting notes
- personal_document: Personal notes, journals, letters, personal content
- article: News articles, blog posts, magazine articles
- report: Technical reports, status reports, analysis reports
- book: Books, chapters, extended literary works
- presentation: Slides, presentation materials
- legal_document: Legal contracts, agreements, court documents
- other: Any other type of document

Document Name: %s

Document Content (first 2000 characters):
%s

Respond with JSON in this exact format:
{
  "primary_type": "category_name",
  "confidence": 0.95,
  "reasoning": "Brief explanation of classification decision"
}`
}

// Research paper unified analysis prompt
func getResearchPaperUnifiedPrompt() string {
	return `You are an expert at analyzing academic research papers. Perform comprehensive analysis on this research paper and extract all relevant information.

Document: %s
Max Entities: %d
Max Citations: %d
Max Topics: %d
Min Confidence: %.2f
%s
%s
Analysis Depth: %s

Document Content:
%s

CRITICAL: You must respond with ONLY valid JSON. Do not include any markdown, explanations, or text outside the JSON structure.

Provide a comprehensive JSON response with the following structure:
{
  "classification": {
    "primary_type": "research_paper",
    "secondary_types": ["article"],
    "confidence": 0.95,
    "features": {
      "has_abstract": 1.0,
      "has_references": 1.0,
      "academic_language": 0.9
    },
    "reasoning": "Contains typical research paper elements"
  },
  "entities": [
    {
      "type": "PERSON",
      "text": "Dr. John Smith",
      "confidence": 0.95,
      "start_offset": 120,
      "end_offset": 133,
      "context": "surrounding text context",
      "metadata": {"role": "author"}
    }
  ],
  "citations": [
    {
      "text": "Smith, J. (2023). Research Methods. Journal Name, 15(2), 123-145.",
      "authors": ["Smith, J."],
      "title": "Research Methods",
      "year": 2023,
      "journal": "Journal Name",
      "volume": "15",
      "issue": "2",
      "pages": "123-145",
      "format": "apa",
      "confidence": 0.95,
      "start_offset": 1500,
      "end_offset": 1580
    }
  ],
  "topics": [
    {
      "name": "Machine Learning",
      "keywords": ["neural networks", "deep learning", "algorithms"],
      "confidence": 0.9,
      "weight": 0.8,
      "description": "Main research topic"
    }
  ],
  "summary": "This research paper discusses...",
  "quality_metrics": {
    "confidence_score": 0.92,
    "completeness_score": 0.88,
    "reliability_score": 0.95
  }
}

Focus on extracting:
- Authors, institutions, researchers (PERSON, ORGANIZATION entities)
- Technical concepts, methodologies (CONCEPT, TECHNOLOGY entities)
- ALL academic citations with complete parsing including (Author Year), Author et al., and full bibliographic references
- Research topics and key themes
- Dates, locations relevant to the research

REMEMBER: Return ONLY valid JSON - no markdown, no explanations, no additional text.`
}

// Business document unified analysis prompt
func getBusinessDocumentUnifiedPrompt() string {
	return `You are an expert at analyzing business documents. Perform comprehensive analysis on this business document and extract all relevant information.

Document: %s
Max Entities: %d
Max Citations: %d
Max Topics: %d
Min Confidence: %.2f
%s
%s
Analysis Depth: %s

Document Content:
%s

Provide a comprehensive JSON response with the same structure as shown in the research paper prompt, but focus on:
- Business entities: companies, executives, stakeholders (PERSON, ORGANIZATION)
- Key metrics, KPIs, financial data (METRIC entities)
- Business concepts, strategies, technologies (CONCEPT, TECHNOLOGY)
- Important dates, deadlines, milestones (DATE)
- Any references or citations to other business documents
- Business topics: strategy, operations, finance, etc.
- Action items and decisions

Adapt the analysis to business context while maintaining the same JSON structure.`
}

// Personal document unified analysis prompt
func getPersonalDocumentUnifiedPrompt() string {
	return `You are an expert at analyzing personal documents. Perform comprehensive analysis on this personal document and extract all relevant information.

Document: %s
Max Entities: %d
Max Citations: %d
Max Topics: %d
Min Confidence: %.2f
%s
%s
Analysis Depth: %s

Document Content:
%s

Provide a comprehensive JSON response with the same structure, but focus on:
- Personal contacts, names mentioned (PERSON)
- Places, locations of significance (LOCATION)
- Important dates, events, anniversaries (DATE)
- Personal interests, hobbies, topics of importance
- Any references to books, articles, or other sources
- Email addresses, contact information (EMAIL)
- Personal topics and themes

Be respectful of privacy while being thorough in extraction.`
}

// Article unified analysis prompt
func getArticleUnifiedPrompt() string {
	return `You are an expert at analyzing news articles and editorial content. Perform comprehensive analysis on this article and extract all relevant information.

Document: %s
Max Entities: %d
Max Citations: %d
Max Topics: %d
Min Confidence: %.2f
%s
%s
Analysis Depth: %s

Document Content:
%s

Provide a comprehensive JSON response focusing on:
- People mentioned in the article (PERSON)
- Organizations, companies, institutions (ORGANIZATION)
- Geographic locations (LOCATION)
- Key dates and events (DATE)
- Article topics and themes
- Sources, quotes, and references
- Technical concepts if applicable (TECHNOLOGY, CONCEPT)

Maintain journalistic accuracy in your extraction.`
}

// Report unified analysis prompt
func getReportUnifiedPrompt() string {
	return `You are an expert at analyzing technical and analytical reports. Perform comprehensive analysis on this report and extract all relevant information.

Document: %s
Max Entities: %d
Max Citations: %d
Max Topics: %d
Min Confidence: %.2f
%s
%s
Analysis Depth: %s

Document Content:
%s

Provide a comprehensive JSON response focusing on:
- Report authors, contributors (PERSON)
- Organizations involved (ORGANIZATION)
- Technical concepts, methodologies (CONCEPT, TECHNOLOGY)
- Metrics, measurements, data points (METRIC)
- Important dates, timelines (DATE)
- Geographic scope (LOCATION)
- References and citations to other reports/sources
- Key findings and recommendations as topics

Focus on technical accuracy and completeness.`
}

// Book unified analysis prompt
func getBookUnifiedPrompt() string {
	return `You are an expert at analyzing books and literary works. Perform comprehensive analysis on this book content and extract all relevant information.

Document: %s
Max Entities: %d
Max Citations: %d
Max Topics: %d
Min Confidence: %.2f
%s
%s
Analysis Depth: %s

Document Content:
%s

Provide a comprehensive JSON response focusing on:
- Character names, real people mentioned (PERSON)
- Places, settings, locations (LOCATION)
- Organizations, institutions (ORGANIZATION)
- Concepts, themes, ideas (CONCEPT)
- Technologies mentioned (TECHNOLOGY)
- Important dates in the narrative (DATE)
- Citations, references to other works
- Major themes and topics
- Literary devices and elements

Adapt the analysis to the book's genre and content type.`
}

// Presentation unified analysis prompt
func getPresentationUnifiedPrompt() string {
	return `You are an expert at analyzing presentation materials and slides. Perform comprehensive analysis on this presentation and extract all relevant information.

Document: %s
Max Entities: %d
Max Citations: %d
Max Topics: %d
Min Confidence: %.2f
%s
%s
Analysis Depth: %s

Document Content:
%s

Provide a comprehensive JSON response focusing on:
- Presenters, speakers, participants (PERSON)
- Organizations, companies mentioned (ORGANIZATION)
- Key concepts, ideas presented (CONCEPT)
- Technologies, tools discussed (TECHNOLOGY)
- Metrics, data points, statistics (METRIC)
- Important dates, schedules (DATE)
- References and sources cited
- Main topics and themes of the presentation

Focus on the key messages and actionable information.`
}

// Legal document unified analysis prompt
func getLegalDocumentUnifiedPrompt() string {
	return `You are an expert at analyzing legal documents. Perform comprehensive analysis on this legal document and extract all relevant information.

Document: %s
Max Entities: %d
Max Citations: %d
Max Topics: %d
Min Confidence: %.2f
%s
%s
Analysis Depth: %s

Document Content:
%s

Provide a comprehensive JSON response focusing on:
- Parties involved, lawyers, judges (PERSON)
- Legal entities, courts, firms (ORGANIZATION)
- Jurisdictions, locations (LOCATION)
- Important dates, deadlines, terms (DATE)
- Legal concepts, terms, precedents (CONCEPT)
- Case citations, legal references
- Legal topics: contract law, criminal law, etc.
- Key terms and obligations

Maintain legal accuracy and precision in extraction.`
}

// Generic unified analysis prompt for other document types
func getGenericUnifiedPrompt() string {
	return `You are an expert document analyst. Perform comprehensive analysis on this document and extract all relevant information.

Document: %s
Max Entities: %d
Max Citations: %d
Max Topics: %d
Min Confidence: %.2f
%s
%s
Analysis Depth: %s

Document Content:
%s

CRITICAL: You must respond with ONLY valid JSON. Do not include any markdown, explanations, or text outside the JSON structure.

Provide a comprehensive JSON response with the standard structure:
{
  "classification": {
    "primary_type": "document",
    "confidence": 0.8,
    "reasoning": "Document classification reasoning"
  },
  "entities": [
    {
      "type": "PERSON",
      "text": "John Smith",
      "confidence": 0.9,
      "start_offset": 100,
      "end_offset": 110,
      "context": "surrounding text"
    }
  ],
  "citations": [
    {
      "text": "Smith (2023)",
      "authors": ["Smith"],
      "year": 2023,
      "format": "apa",
      "confidence": 0.9,
      "start_offset": 200,
      "end_offset": 212
    }
  ],
  "topics": [
    {
      "name": "Topic Name",
      "keywords": ["keyword1", "keyword2"],
      "confidence": 0.8,
      "weight": 0.7,
      "description": "Topic description"
    }
  ],
  "summary": "Document summary",
  "quality_metrics": {
    "confidence_score": 0.85,
    "completeness_score": 0.80,
    "reliability_score": 0.90
  }
}

Extract all relevant:
- People, organizations, locations (PERSON, ORGANIZATION, LOCATION entities)
- Dates, emails, contacts (DATE, EMAIL, CONTACT entities)
- Concepts, technologies, metrics (CONCEPT, TECHNOLOGY, METRIC entities)
- ANY references or citations including in-text citations like (Author Year), Author et al., and bibliographic references
- Key topics and themes
- Important information based on document context

REMEMBER: Return ONLY valid JSON - no markdown, no explanations, no additional text.`
}
