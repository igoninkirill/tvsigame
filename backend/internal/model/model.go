// Package model — нормализованные структуры данных пака вопросов.
// Это «каноническое» представление, которое хранится в packs.parsed_data
// и используется игровым движком. Парсер .siq преобразует XML в эту модель.
package model

// Atom — один фрагмент сценария вопроса (текст, изображение, аудио, видео).
// Порядок атомов важен: пакет задаёт последовательность предъявления.
type Atom struct {
	Kind     string  `json:"kind"`            // text | image | audio | video | say | marker | ...
	Text     string  `json:"text,omitempty"`  // текст фрагмента (для text/say)
	MediaURL string  `json:"mediaUrl,omitempty"` // относительный URL медиа (/media/...)
	Time     float64 `json:"time,omitempty"`  // длительность фрагмента из пака, сек (если задана)
}

// Question — нормализованный вопрос.
type Question struct {
	ID           string            `json:"id"` // "r{round}/t{theme}/q{idx}" — стабильный внутри пака
	Price        int64             `json:"price"`
	Type         string            `json:"type"` // regular | auction | cat | noRisk | forAll | unknown:<name>
	TypeParams   map[string]string `json:"typeParams,omitempty"`
	Scenario     []Atom            `json:"scenario"`
	// AnswerScenario — фрагменты, показываемые при раскрытии правильного
	// ответа (формат params новых пакетов: param name="answer").
	AnswerScenario []Atom          `json:"answerScenario,omitempty"`
	Answers      []string          `json:"answers"`
	WrongAnswers []string          `json:"wrongAnswers,omitempty"`
	Comment      string            `json:"comment,omitempty"`
}

// QuestionText — конкатенация текстовых атомов (для быстрого показа).
func (q Question) QuestionText() string {
	out := ""
	for _, a := range q.Scenario {
		if a.Kind == "text" || a.Kind == "say" {
			if out != "" && a.Text != "" {
				out += " "
			}
			out += a.Text
		}
	}
	return out
}

func (q Question) RightAnswer() string {
	if len(q.Answers) == 0 {
		return ""
	}
	return q.Answers[0]
}

type Theme struct {
	Name      string     `json:"name"`
	Questions []Question `json:"questions"`
}

type Round struct {
	Name   string  `json:"name"`
	Type   string  `json:"type"` // standard | final
	Themes []Theme `json:"themes"`
}

type Pack struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Author     string  `json:"author"`
	Version    string  `json:"version"`
	Date       string  `json:"date,omitempty"`
	Difficulty string  `json:"difficulty,omitempty"`
	LogoURL    string  `json:"logoUrl,omitempty"`
	Rounds     []Round `json:"rounds"`
}
