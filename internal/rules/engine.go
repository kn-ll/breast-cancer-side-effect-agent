package rules

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"breast-cancer-side-effect-agent/internal/domain"
)

const Version = "breast-side-effect-rules-v0.1.0"

type Rule struct {
	ID       string
	Name     string
	Level    domain.RiskLevel
	Priority int

	Keywords []string
	Signals  []string
	MinTemp  *float64
	MaxTemp  *float64

	ReasonTemplate string
}

type Engine struct {
	rules []Rule
}

func NewEngine() *Engine {
	rules := builtinRules()
	sort.SliceStable(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})
	return &Engine{rules: rules}
}

func (e *Engine) Evaluate(description string, analysis domain.AIAnalysis) (domain.Advice, domain.Evidence, domain.RuleSource) {
	text := normalize(description + " " + analysis.Summary + " " + strings.Join(analysis.Symptoms, " ") + " " + strings.Join(analysis.SeveritySignals, " "))
	temps := ExtractTemperatures(description)
	if analysis.TemperatureCelsius != nil {
		temps = append(temps, *analysis.TemperatureCelsius)
	}

	for _, rule := range e.rules {
		matched := matchedTerms(rule, text, temps)
		if len(matched) == 0 {
			continue
		}
		advice := buildAdvice(rule.Level)
		evidence := domain.Evidence{
			MatchedRuleID:   rule.ID,
			MatchedRuleName: rule.Name,
			MatchedKeywords: matched,
			Reason:          fmt.Sprintf(rule.ReasonTemplate, strings.Join(matched, ", ")),
			AISummary:       analysis.Summary,
			AISignals:       analysis.SeveritySignals,
		}
		source := domain.RuleSource{
			Engine:   "rules",
			Version:  Version,
			RuleID:   rule.ID,
			RuleName: rule.Name,
			Source:   "prototype_clinical_safety_rules",
		}
		return advice, evidence, source
	}

	advice := buildAdvice(domain.RiskLow)
	evidence := domain.Evidence{
		MatchedRuleID:   "LOW_DEFAULT_OBSERVE",
		MatchedRuleName: "未命中明确风险规则",
		MatchedKeywords: []string{"default_observe"},
		Reason:          "未发现高风险或中风险关键词，当前按低风险观察处理；若症状加重或出现新症状，请重新评估。",
		AISummary:       analysis.Summary,
		AISignals:       analysis.SeveritySignals,
	}
	source := domain.RuleSource{
		Engine:   "rules",
		Version:  Version,
		RuleID:   evidence.MatchedRuleID,
		RuleName: evidence.MatchedRuleName,
		Source:   "prototype_clinical_safety_rules",
	}
	return advice, evidence, source
}

func builtinRules() []Rule {
	t38 := 38.0
	t375 := 37.5
	return []Rule{
		{
			ID:             "H001_BREATHING_OR_CHEST",
			Name:           "呼吸或胸痛急症风险",
			Level:          domain.RiskHigh,
			Priority:       10,
			Keywords:       []string{"呼吸困难", "胸痛", "喘不过气", "严重气短", "气短明显", "呼吸急促"},
			Signals:        []string{"shortness_of_breath", "chest_pain", "breathing_distress"},
			ReasonTemplate: "出现呼吸或胸痛相关高风险信号：%s。",
		},
		{
			ID:             "H002_HIGH_FEVER_OR_INFECTION",
			Name:           "高热或感染风险",
			Level:          domain.RiskHigh,
			Priority:       20,
			Keywords:       []string{"寒战", "感染", "伤口化脓", "化脓", "红肿热痛", "高烧", "高热"},
			Signals:        []string{"fever_38_plus", "chills", "possible_infection"},
			MinTemp:        &t38,
			ReasonTemplate: "体温或感染相关信号达到高风险阈值：%s。",
		},
		{
			ID:             "H003_ALLERGY_BLEEDING_NEURO",
			Name:           "严重过敏、出血或神经系统风险",
			Level:          domain.RiskHigh,
			Priority:       30,
			Keywords:       []string{"严重过敏", "脸肿", "嘴唇肿", "喉咙肿", "喉咙紧", "出血不止", "意识模糊", "晕厥", "昏倒", "抽搐"},
			Signals:        []string{"severe_allergy", "uncontrolled_bleeding", "confusion", "syncope"},
			ReasonTemplate: "出现严重过敏、出血或神经系统相关高风险信号：%s。",
		},
		{
			ID:             "H004_DEHYDRATION_OR_PERSISTENT_VOMITING",
			Name:           "持续呕吐或脱水风险",
			Level:          domain.RiskHigh,
			Priority:       40,
			Keywords:       []string{"持续呕吐", "一直吐", "无法进食", "不能喝水", "无法喝水", "尿量明显减少", "尿很少", "脱水"},
			Signals:        []string{"persistent_vomiting", "cannot_keep_fluids", "dehydration"},
			ReasonTemplate: "出现持续呕吐或脱水相关高风险信号：%s。",
		},
		{
			ID:             "M002_MILD_FEVER_OR_WORSENING_PAIN",
			Name:           "低热或症状加重",
			Level:          domain.RiskMedium,
			Priority:       100,
			Keywords:       []string{"低烧", "发烧", "发热", "疼痛加重", "越来越痛", "乏力加重", "越来越累"},
			Signals:        []string{"fever_37_5_to_38", "worsening_pain", "worsening_fatigue"},
			MinTemp:        &t375,
			MaxTemp:        &t38,
			ReasonTemplate: "出现低热或症状加重相关信号：%s。",
		},
		{
			ID:             "M001_GI_SKIN_NEURO",
			Name:           "胃肠道、皮肤或神经症状",
			Level:          domain.RiskMedium,
			Priority:       110,
			Keywords:       []string{"恶心", "呕吐", "腹泻", "拉肚子", "皮疹", "红疹", "口腔溃疡", "嘴破", "手脚麻", "手脚麻木", "头晕"},
			Signals:        []string{"nausea", "vomiting", "diarrhea", "rash", "mouth_sore", "neuropathy", "dizziness"},
			ReasonTemplate: "出现需要联系团队或密切观察的中风险症状：%s。",
		},
		{
			ID:             "M003_LYMPHEDEMA_OR_SWELLING",
			Name:           "肿胀或疑似淋巴水肿",
			Level:          domain.RiskMedium,
			Priority:       120,
			Keywords:       []string{"手臂肿", "手臂肿胀", "腋窝肿", "腋窝肿胀", "乳房肿", "乳房肿胀", "淋巴水肿"},
			Signals:        []string{"lymphedema", "arm_swelling", "breast_swelling", "axilla_swelling"},
			ReasonTemplate: "出现肿胀或疑似淋巴水肿相关信号：%s。",
		},
		{
			ID:             "L001_MILD_EXPECTED_EFFECTS",
			Name:           "轻微常见治疗相关反应",
			Level:          domain.RiskLow,
			Priority:       200,
			Keywords:       []string{"轻微乏力", "有点乏力", "轻微脱发", "脱发", "食欲下降", "睡眠差", "睡不好", "潮热", "轻微疲劳"},
			Signals:        []string{"mild_fatigue", "hair_loss", "poor_sleep", "hot_flash", "low_appetite"},
			ReasonTemplate: "出现轻微、可继续记录观察的症状：%s。",
		},
	}
}

func matchedTerms(rule Rule, text string, temps []float64) []string {
	seen := map[string]struct{}{}
	var matches []string
	add := func(v string) {
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		matches = append(matches, v)
	}

	for _, keyword := range rule.Keywords {
		if strings.Contains(text, normalize(keyword)) {
			add(keyword)
		}
	}
	for _, signal := range rule.Signals {
		if strings.Contains(text, normalize(signal)) {
			add(signal)
		}
	}
	for _, temp := range temps {
		if rule.MinTemp != nil && temp < *rule.MinTemp {
			continue
		}
		if rule.MaxTemp != nil && temp >= *rule.MaxTemp {
			continue
		}
		if rule.MinTemp != nil || rule.MaxTemp != nil {
			add(fmt.Sprintf("%.1f°C", temp))
		}
	}
	return matches
}

func buildAdvice(level domain.RiskLevel) domain.Advice {
	switch level {
	case domain.RiskHigh:
		return domain.Advice{
			RiskLevel:   level,
			ContactTeam: true,
			Urgency:     "immediate_or_24h",
			NextSteps: []string{
				"立即线下就医；若症状危急，请拨打当地急救电话。",
				"24 小时内联系肿瘤/乳腺治疗团队。",
				"记录最高体温、症状开始时间、当前用药、饮水和尿量情况。",
			},
		}
	case domain.RiskMedium:
		return domain.Advice{
			RiskLevel:   level,
			ContactTeam: true,
			Urgency:     "contact_team_or_watch_closely",
			NextSteps: []string{
				"建议联系治疗团队确认是否需要处理。",
				"密切观察 24-48 小时，并记录症状变化。",
				"如果出现高热、呼吸困难、胸痛、持续呕吐或意识异常，请立即线下就医。",
			},
		}
	default:
		return domain.Advice{
			RiskLevel:   domain.RiskLow,
			ContactTeam: false,
			Urgency:     "observe_and_record",
			NextSteps: []string{
				"继续观察与记录症状、时间和诱因。",
				"保持治疗团队建议的日常护理方式。",
				"如果症状加重或出现新症状，请重新评估或联系团队。",
			},
		}
	}
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

var tempPattern = regexp.MustCompile(`([3-4][0-9](?:\.\d+)?)\s*(?:°c|℃|度|c)?`)

func ExtractTemperatures(text string) []float64 {
	matches := tempPattern.FindAllStringSubmatch(strings.ToLower(text), -1)
	var temps []float64
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		v, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			continue
		}
		if v >= 35 && v <= 43 {
			temps = append(temps, v)
		}
	}
	return temps
}
