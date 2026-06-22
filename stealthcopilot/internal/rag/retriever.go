package rag

import (
	"sort"
	"strings"
	"unicode"

	"github.com/zhaoyta/stealthcopilot/internal/resume"
)

// TopK 是 RAG 检索返回的最大简历片段数。
const TopK = 3

const fullResumeContextMaxChars = 12000

// Retriever 在当前激活简历的向量库中检索与查询最相关的段落。
// 底层调用 resume.Manager 的 Search() 方法，复用已有向量检索能力。
type Retriever struct {
	manager *resume.Manager
}

// NewRetriever 创建 Retriever。manager 通过 resume.Service.InternalManager() 获取。
func NewRetriever(manager *resume.Manager) *Retriever {
	return &Retriever{manager: manager}
}

// RetrieveResult 是 RAG 检索的返回结构。
type RetrieveResult struct {
	// Chunks 是相关简历片段列表（按相似度降序，最多 TopK 条）。
	Chunks []string
	// HasActiveResume 为 false 表示无激活简历，回答生成应降级为通用回答。
	HasActiveResume bool
}

// Retrieve 对 queryText 生成 embedding，在激活简历向量库中检索 TopK 相关段落。
//   - 无激活简历：返回 HasActiveResume=false，Chunks 为空
//   - embedding 不可用或检索出错：返回空 Chunks，调用方降级处理
func (r *Retriever) Retrieve(queryText string) RetrieveResult {
	return r.RetrieveWithContext(queryText, "", nil)
}

// RetrieveWithContext uses the current question plus recent conversation text
// to resolve follow-up references such as "this project".
func (r *Retriever) RetrieveWithContext(srcText string, dstText string, historyTexts []string) RetrieveResult {
	if !r.manager.HasActiveResume() {
		return RetrieveResult{HasActiveResume: false}
	}

	scopeTexts := retrievalScopeTexts(srcText, dstText, historyTexts)
	if needsProjectContext(srcText, dstText) {
		if projectText, ok := r.bestActiveResumeProjectContext(scopeTexts...); ok {
			return RetrieveResult{HasActiveResume: true, Chunks: []string{projectText}}
		}
	}

	if needsFullResumeContext(scopeTexts...) {
		if fullText, ok := r.fullActiveResumeContext(); ok {
			return RetrieveResult{HasActiveResume: true, Chunks: []string{fullText}}
		}
	}

	queries := retrievalQueries(r.manager.ActiveResumeLanguage(), srcText, dstText)
	if len(queries) == 0 {
		return RetrieveResult{HasActiveResume: true, Chunks: nil}
	}

	results, err := r.searchQueries(queries, TopK)
	if err != nil {
		// embedding 未就绪（ErrModelNotReady）或其他错误：降级为无 context 回答
		return RetrieveResult{HasActiveResume: true, Chunks: nil}
	}

	chunks := make([]string, 0, len(results))
	for _, res := range results {
		chunks = append(chunks, res.ChunkText)
	}
	return RetrieveResult{HasActiveResume: true, Chunks: chunks}
}

func retrievalScopeTexts(srcText, dstText string, historyTexts []string) []string {
	texts := make([]string, 0, 2+len(historyTexts))
	for _, text := range []string{srcText, dstText} {
		if strings.TrimSpace(text) != "" {
			texts = append(texts, text)
		}
	}
	for _, text := range historyTexts {
		if strings.TrimSpace(text) != "" {
			texts = append(texts, text)
		}
	}
	return texts
}

func (r *Retriever) fullActiveResumeContext() (string, bool) {
	text, ok, err := r.manager.ActiveResumeText()
	if err != nil || !ok {
		return "", false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	if len([]rune(text)) > fullResumeContextMaxChars {
		runes := []rune(text)
		text = string(runes[:fullResumeContextMaxChars]) + "\n\n[简历内容较长，已截取前部核心内容。]"
	}
	return text, true
}

func (r *Retriever) bestActiveResumeProjectContext(texts ...string) (string, bool) {
	fullText, ok, err := r.manager.ActiveResumeText()
	if err != nil || !ok {
		return "", false
	}
	sections := splitProjectSections(fullText)
	if len(sections) == 0 {
		return "", false
	}
	var best projectSection
	bestScore := 0
	for _, section := range sections {
		score := projectSectionScore(section, texts)
		if score > bestScore {
			best = section
			bestScore = score
		}
	}
	if bestScore == 0 {
		return "", false
	}
	return strings.TrimSpace(best.Text), true
}

func needsProjectContext(texts ...string) bool {
	for _, text := range texts {
		if projectSpecificQuestion(text) {
			return true
		}
	}
	return false
}

func needsFullResumeContext(texts ...string) bool {
	for _, text := range texts {
		if broadResumeQuestion(text) {
			return true
		}
	}
	return false
}

func projectSpecificQuestion(text string) bool {
	text = normalizeQuestionText(text)
	if text == "" {
		return false
	}
	if broadProjectOverviewQuestion(text) {
		return false
	}
	if hasAnyPhrase(text,
		"in this project",
		"in that project",
		"for this project",
		"for that project",
		"this project",
		"that project",
		"the project",
		"your role in",
		"what was your role",
		"what did you do in",
		"what did you build in",
		"这个项目",
		"该项目",
		"这个系统",
		"该系统",
		"这个平台",
		"该平台",
		"你在项目中",
		"你在这个项目",
		"你负责什么",
		"承担什么",
		"怎么设计",
		"如何设计",
	) {
		return true
	}
	return mentionsNamedProject(text)
}

func broadProjectOverviewQuestion(text string) bool {
	return hasAnyPhrase(text,
		"your project experience",
		"your projects",
		"projects you have done",
		"introduce your projects",
		"tell me about your projects",
		"介绍一下你的项目",
		"介绍你的项目",
		"项目经验",
		"项目经历",
		"做过哪些项目",
	)
}

func broadResumeQuestion(text string) bool {
	text = normalizeQuestionText(text)
	if text == "" {
		return false
	}
	return hasAnyPhrase(text,
		"tell me about yourself",
		"introduce yourself",
		"self introduction",
		"walk me through your resume",
		"walk me through your background",
		"tell me about your background",
		"tell me about your experience",
		"describe your experience",
		"your work experience",
		"your project experience",
		"system design",
		"architecture",
		"high performance",
		"scalable",
		"scalability",
		"介绍一下你自己",
		"自我介绍",
		"说说你自己",
		"讲讲你自己",
		"介绍一下你的背景",
		"介绍你的背景",
		"介绍一下你的经历",
		"介绍你的经历",
		"介绍一下你的项目",
		"介绍你的项目",
		"项目经验",
		"工作经历",
		"系统设计",
		"体系结构",
		"架构经验",
		"高性能",
		"高并发",
	)
}

func normalizeQuestionText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	var b strings.Builder
	prevSpace := false
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r > unicode.MaxASCII {
			b.WriteRune(r)
			prevSpace = false
			continue
		}
		if !prevSpace {
			b.WriteByte(' ')
			prevSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func hasAnyPhrase(text string, phrases ...string) bool {
	for _, phrase := range phrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

type projectSection struct {
	Title string
	Text  string
}

func splitProjectSections(text string) []projectSection {
	lines := normalizeResumeLines(text)
	var sections []projectSection
	var current []string
	inProjects := false

	flush := func() {
		if len(current) == 0 {
			return
		}
		title := strings.TrimSpace(current[0])
		body := strings.TrimSpace(strings.Join(current, "\n"))
		if title != "" && body != "" {
			sections = append(sections, projectSection{Title: title, Text: body})
		}
		current = nil
	}

	for _, line := range lines {
		if isProjectAreaHeading(line) {
			inProjects = true
			flush()
			continue
		}
		if inProjects && isMajorResumeHeading(line) && !isProjectAreaHeading(line) {
			flush()
			inProjects = false
			continue
		}
		if !inProjects {
			continue
		}
		if isLikelyProjectTitle(line) {
			flush()
		}
		current = append(current, line)
	}
	flush()

	if len(sections) > 0 {
		return sections
	}
	return splitLooseProjectSections(lines)
}

func splitLooseProjectSections(lines []string) []projectSection {
	var sections []projectSection
	var current []string
	flush := func() {
		if len(current) >= 2 {
			sections = append(sections, projectSection{
				Title: strings.TrimSpace(current[0]),
				Text:  strings.TrimSpace(strings.Join(current, "\n")),
			})
		}
		current = nil
	}
	for _, line := range lines {
		if isLikelyProjectTitle(line) {
			flush()
		}
		if current != nil || isLikelyProjectTitle(line) {
			current = append(current, line)
		}
	}
	flush()
	return sections
}

func normalizeResumeLines(text string) []string {
	raw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(strings.Trim(line, "•-—* \t"))
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func isProjectAreaHeading(line string) bool {
	normalized := normalizeQuestionText(line)
	return hasAnyPhrase(normalized,
		"project experience",
		"projects",
		"selected projects",
		"representative projects",
		"项目经历",
		"项目经验",
		"代表项目",
		"核心项目",
	)
}

func isMajorResumeHeading(line string) bool {
	normalized := normalizeQuestionText(line)
	if runeLen(normalized) > 24 {
		return false
	}
	return hasAnyPhrase(normalized,
		"education",
		"skills",
		"technical skills",
		"work experience",
		"professional experience",
		"summary",
		"certifications",
		"教育经历",
		"教育背景",
		"专业技能",
		"技能清单",
		"工作经历",
		"实习经历",
		"个人总结",
		"证书",
	)
}

func isLikelyProjectTitle(line string) bool {
	normalized := normalizeQuestionText(line)
	if normalized == "" || isProjectAreaHeading(line) || isMajorResumeHeading(line) || runeLen(normalized) > 90 {
		return false
	}
	return hasAnyPhrase(normalized,
		"project",
		"system",
		"platform",
		"service",
		"app",
		"application",
		"payment",
		"order",
		"recommendation",
		"risk",
		"项目",
		"系统",
		"平台",
		"服务",
		"应用",
		"商城",
		"支付",
		"订单",
		"推荐",
		"风控",
		"履约",
	)
}

func projectSectionScore(section projectSection, queries []string) int {
	sectionText := normalizeQuestionText(section.Title + " " + section.Text)
	titleText := normalizeQuestionText(section.Title)
	score := 0
	for _, query := range queries {
		queryText := normalizeQuestionText(query)
		if queryText == "" {
			continue
		}
		if strings.Contains(queryText, titleText) || strings.Contains(sectionText, queryText) {
			score += 20
		}
		for _, keyword := range queryKeywords(queryText) {
			if keyword == "" {
				continue
			}
			if strings.Contains(titleText, keyword) {
				score += 5
			} else if strings.Contains(sectionText, keyword) {
				score += 2
			}
		}
	}
	return score
}

func queryKeywords(text string) []string {
	stop := map[string]bool{
		"the": true, "this": true, "that": true, "your": true, "you": true, "what": true, "how": true, "why": true,
		"was": true, "were": true, "did": true, "do": true, "in": true, "for": true, "about": true, "project": true,
		"tell": true, "me": true, "介绍": true, "一下": true, "这个": true, "那个": true, "项目": true, "系统": true,
		"你": true, "的": true, "了": true, "吗": true, "什么": true, "怎么": true, "如何": true,
	}
	fields := strings.Fields(text)
	keywords := make([]string, 0, len(fields))
	for _, field := range fields {
		if stop[field] || runeLen(field) < 2 {
			continue
		}
		keywords = append(keywords, field)
		if containsNonASCII(field) {
			for _, term := range []string{"支付", "订单", "推荐", "风控", "履约", "商城", "缓存", "高并发", "高性能", "架构", "系统", "平台", "服务"} {
				if strings.Contains(field, term) {
					keywords = append(keywords, term)
				}
			}
		}
	}
	return keywords
}

func mentionsNamedProject(text string) bool {
	if !hasAnyPhrase(text, "project", "system", "platform", "项目", "系统", "平台") {
		return false
	}
	for _, keyword := range queryKeywords(text) {
		if runeLen(keyword) >= 3 {
			return true
		}
	}
	return false
}

func runeLen(text string) int {
	return len([]rune(text))
}

func containsNonASCII(text string) bool {
	for _, r := range text {
		if r > unicode.MaxASCII {
			return true
		}
	}
	return false
}

func retrievalQueries(language resume.ResumeLanguage, srcText string, dstText string) []string {
	srcText = strings.TrimSpace(srcText)
	dstText = strings.TrimSpace(dstText)
	add := func(out []string, text string) []string {
		if text == "" {
			return out
		}
		for _, existing := range out {
			if existing == text {
				return out
			}
		}
		return append(out, text)
	}

	var queries []string
	switch language {
	case resume.ResumeLanguageEN:
		queries = add(queries, srcText)
		queries = add(queries, dstText)
	case resume.ResumeLanguageZH:
		queries = add(queries, dstText)
		queries = add(queries, srcText)
	default:
		queries = add(queries, srcText)
		queries = add(queries, dstText)
	}
	return queries
}

func (r *Retriever) searchQueries(queries []string, topK int) ([]resume.SearchResult, error) {
	merged := make(map[int64]resume.SearchResult)
	for _, query := range queries {
		results, err := r.manager.Search(query, topK)
		if err != nil {
			return nil, err
		}
		for _, result := range results {
			if existing, ok := merged[result.ChunkID]; !ok || result.Score > existing.Score {
				merged[result.ChunkID] = result
			}
		}
	}

	results := make([]resume.SearchResult, 0, len(merged))
	for _, result := range merged {
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}
