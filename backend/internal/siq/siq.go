// Package siq — импортёр пакетов вопросов формата .siq (SIGame / SIBrowser).
//
// .siq — это ZIP-архив с content.xml и медиафайлами (Images/, Audio/, Video/).
// Импортёр:
//   - проверяет архив (ZIP Slip, архивные бомбы, лимиты размера);
//   - парсит content.xml (Go encoding/xml не обрабатывает внешние сущности,
//     поэтому XXE-атаки исключены по построению);
//   - извлекает только реально используемые медиафайлы;
//   - возвращает нормализованную модель пакета (см. package model).
//
// Поддерживаются версии формата v5 (приоритет) и v4 (совместимость).
package siq

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/krutokir/svoya-igra/server/internal/model"
)

// Лимиты безопасности архива.
const (
	maxArchiveEntries   = 5000
	maxTotalUncompressed = 2 << 30 // 2 ГБ
	maxEntryUncompressed = 256 << 20 // 256 МБ на файл
	maxContentXML        = 64 << 20  // 64 МБ
)

// --- XML-представление формата .siq ---

type xmlParam struct {
	Name  string `xml:"name,attr"`
	Type  string `xml:"type,attr"`
	Value string `xml:"value,attr"`
	Inner string `xml:",innerxml"`
}

type xmlType struct {
	Name   string     `xml:"name,attr"`
	Params []xmlParam `xml:"param"`
}

type xmlAtom struct {
	Type string  `xml:"type,attr"`
	Time float64 `xml:"time,attr"`
	Text string  `xml:",chardata"`
}

// xmlItem — элемент контента нового формата пакетов (params>param>item):
// <item type="image" isRef="True" placement="background">file.jpg</item>.
type xmlItem struct {
	Type      string  `xml:"type,attr"`
	IsRef     string  `xml:"isRef,attr"`
	Placement string  `xml:"placement,attr"`
	Time      float64 `xml:"time,attr"`
	Text      string  `xml:",chardata"`
}

// xmlItems — обёртка для разбора innerxml контентного параметра.
type xmlItems struct {
	Items []xmlItem `xml:"item"`
}

type xmlNumberSet struct {
	Minimum float64 `xml:"minimum,attr"`
	Maximum float64 `xml:"maximum,attr"`
}

type xmlQuestion struct {
	Price    int64      `xml:"price,attr"`
	TypeAttr string     `xml:"type,attr"` // новый формат: <question type="secret">
	Type     *xmlType   `xml:"type"`      // классический формат: <type name="auction"/>
	Scenario []xmlAtom  `xml:"scenario>atom"`
	Params   []xmlParam `xml:"params>param"` // новый формат контента
	Right    []string   `xml:"right>answer"`
	Wrong    []string   `xml:"wrong>answer"`
	Comment  string     `xml:"info>comments"`
}

type xmlTheme struct {
	Name      string       `xml:"name,attr"`
	Questions []xmlQuestion `xml:"questions>question"`
}

type xmlRound struct {
	Name   string     `xml:"name,attr"`
	Type   string     `xml:"type,attr"`
	Themes []xmlTheme `xml:"themes>theme"`
}

type xmlPackage struct {
	Name       string     `xml:"name,attr"`
	Version    string     `xml:"version,attr"`
	Date       string     `xml:"date,attr"`
	Difficulty string     `xml:"difficulty,attr"`
	Logo       string     `xml:"logo"`
	Authors    []string   `xml:"info>authors>author"`
	Rounds     []xmlRound `xml:"rounds>round"`
}

// Result — результат импорта.
type Result struct {
	Pack     model.Pack
	Warnings []string
}

// Import разбирает .siq из r и сохраняет медиа в mediaDir/{packID}/.
// Возвращает нормализованный пак с относительными URL медиа (/media/{packID}/...).
func Import(r io.ReaderAt, size int64, packID, mediaDir string) (*Result, error) {
	if size > maxTotalUncompressed {
		return nil, fmt.Errorf("архив слишком большой (%.1f МБ)", float64(size)/(1<<20))
	}
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать ZIP-архив: %w", err)
	}
	if len(zr.File) > maxArchiveEntries {
		return nil, fmt.Errorf("слишком много файлов в архиве (>%d)", maxArchiveEntries)
	}

	// Индекс файлов архива и поиск content.xml.
	var contentEntry *zip.File
	byName := map[string]*zip.File{}
	var total int64
	for _, f := range zr.File {
		clean := path.Clean(strings.ReplaceAll(f.Name, "\\", "/"))
		if strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "..") || f.FileInfo().IsDir() {
			continue // ZIP Slip / директории
		}
		total += int64(f.UncompressedSize64)
		if f.UncompressedSize64 > maxEntryUncompressed {
			return nil, fmt.Errorf("файл %s превышает лимит размера", clean)
		}
		if total > maxTotalUncompressed {
			return nil, fmt.Errorf("суммарный размер архива превышает лимит")
		}
		byName[strings.ToLower(clean)] = f
		// Sibrowser и ряд редакторов хранят имена файлов в архиве
		// URL-кодированными (%D0%BB...), а в XML ссылаются на декодированные.
		// Индексируем оба варианта.
		if dec, err := url.PathUnescape(clean); err == nil && dec != clean {
			if _, exists := byName[strings.ToLower(dec)]; !exists {
				byName[strings.ToLower(dec)] = f
			}
		}
		base := strings.ToLower(path.Base(clean))
		if base == "content.xml" && contentEntry == nil {
			contentEntry = f
		}
	}
	if contentEntry == nil {
		return nil, fmt.Errorf("в архиве не найден content.xml — это не пакет .siq")
	}

	pkg, err := parseContentXML(contentEntry)
	if err != nil {
		return nil, err
	}

	// Извлечение медиа.
	extractor := &mediaExtractor{
		byName:  byName,
		destDir: filepath.Join(mediaDir, packID),
		urlBase: "/media/" + packID + "/",
		used:    map[string]string{},
	}
	if err := os.MkdirAll(extractor.destDir, 0o755); err != nil {
		return nil, fmt.Errorf("не удалось создать каталог медиа: %w", err)
	}

	res := &Result{Pack: model.Pack{
		ID:         packID,
		Name:       strings.TrimSpace(pkg.Name),
		Author:     strings.Join(pkg.Authors, ", "),
		Version:    pkg.Version,
		Date:       pkg.Date,
		Difficulty: pkg.Difficulty,
	}}
	if res.Pack.Name == "" {
		res.Pack.Name = "Без названия"
	}

	if logo := strings.TrimSpace(pkg.Logo); logo != "" {
		if u, warn := extractor.Resolve(logo); u != "" {
			res.Pack.LogoURL = u
		} else if warn != "" {
			res.Warnings = append(res.Warnings, warn)
		}
	}

	for ri, xr := range pkg.Rounds {
		round := model.Round{Name: strings.TrimSpace(xr.Name), Type: "standard"}
		if strings.EqualFold(xr.Type, "final") {
			round.Type = "final"
		}
		if round.Name == "" {
			round.Name = fmt.Sprintf("Раунд %d", ri+1)
		}
		for ti, xt := range xr.Themes {
			theme := model.Theme{Name: strings.TrimSpace(xt.Name)}
			if theme.Name == "" {
				theme.Name = fmt.Sprintf("Тема %d", ti+1)
			}
			for qi, xq := range xt.Questions {
				q := model.Question{
					ID:           fmt.Sprintf("r%d/t%d/q%d", ri, ti, qi),
					Price:        xq.Price,
					Answers:      cleanList(xq.Right),
					WrongAnswers: cleanList(xq.Wrong),
					Comment:      strings.TrimSpace(xq.Comment),
				}
				q.Type, q.TypeParams = resolveQuestionType(&xq)
				// Классический формат: <scenario><atom>.
				for _, xa := range xq.Scenario {
					atom := model.Atom{Kind: normalizeAtomKind(xa.Type), Text: strings.TrimSpace(xa.Text), Time: xa.Time}
					if !materializeAtom(&atom, extractor, res, q.ID) {
						continue
					}
					q.Scenario = append(q.Scenario, atom)
				}
				// Новый формат: <params><param name="question|answer" type="content"><item>.
				for _, xp := range xq.Params {
					var target *[]model.Atom
					switch strings.ToLower(strings.TrimSpace(xp.Name)) {
					case "question":
						target = &q.Scenario
					case "answer":
						target = &q.AnswerScenario
					default:
						continue
					}
					for _, atom := range parseContentParam(xp) {
						if !materializeAtom(&atom, extractor, res, q.ID) {
							continue
						}
						*target = append(*target, atom)
					}
				}
				// «Секрет» с выбором стоимости: берём минимум numberSet
				// (в SIGame при совпадающих границах это фиксированная цена).
				if q.Type == "cat" {
					if price, ok := numberSetMinimum(&xq); ok && price > 0 {
						q.Price = price
					}
				}
				theme.Questions = append(theme.Questions, q)
			}
			round.Themes = append(round.Themes, theme)
		}
		res.Pack.Rounds = append(res.Pack.Rounds, round)
	}

	if len(res.Pack.Rounds) == 0 {
		return nil, fmt.Errorf("в пакете нет раундов")
	}
	sort.Strings(res.Warnings)
	res.Warnings = dedupe(res.Warnings)
	return res, nil
}

func parseContentXML(f *zip.File) (*xmlPackage, error) {
	if f.UncompressedSize64 > maxContentXML {
		return nil, fmt.Errorf("content.xml слишком большой")
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть content.xml: %w", err)
	}
	defer rc.Close()

	var pkg xmlPackage
	dec := xml.NewDecoder(io.LimitReader(rc, maxContentXML))
	dec.Strict = false // реальные пакеты бывают небрежными
	if err := dec.Decode(&pkg); err != nil {
		return nil, fmt.Errorf("ошибка разбора content.xml: %w", err)
	}
	v := pkg.Version
	if v != "" && !strings.HasPrefix(v, "5") && !strings.HasPrefix(v, "4") {
		return nil, fmt.Errorf("неподдерживаемая версия формата пакета: %s (поддерживаются v4 и v5)", v)
	}
	return &pkg, nil
}

// mapQuestionType приводит типы вопросов .siq к каноническим.
// Неизвестные типы не теряются: сохраняются как unknown:<имя> с параметрами.
func mapQuestionType(t *xmlType) (string, map[string]string) {
	if t == nil {
		return "regular", nil
	}
	params := map[string]string{}
	for _, p := range t.Params {
		v := p.Value
		if v == "" {
			v = strings.TrimSpace(p.Inner)
		}
		params[p.Name] = v
	}
	return canonicalQuestionType(t.Name, params)
}

// resolveQuestionType определяет тип вопроса: новый формат задаёт его
// атрибутом <question type="...">, классический — дочерним <type name="...">.
func resolveQuestionType(xq *xmlQuestion) (string, map[string]string) {
	if name := strings.TrimSpace(xq.TypeAttr); name != "" {
		params := map[string]string{}
		for _, p := range xq.Params {
			// Контентные параметры — это сценарий, а не параметры типа.
			if strings.EqualFold(p.Type, "content") {
				continue
			}
			v := p.Value
			if v == "" && !strings.Contains(p.Inner, "<") {
				v = strings.TrimSpace(p.Inner)
			}
			params[p.Name] = v
		}
		return canonicalQuestionType(name, params)
	}
	return mapQuestionType(xq.Type)
}

func canonicalQuestionType(name string, params map[string]string) (string, map[string]string) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "default", "regular", "simple":
		return "regular", params
	case "stake", "auction":
		return "auction", params
	case "secret", "cat", "bagcat", "secretwithprice", "secretpublicprice", "secretnoquestion":
		return "cat", params
	case "norisk", "sponsored", "forself", "simplewithprice":
		return "noRisk", params
	case "forall", "foreveryone", "everyone":
		return "forAll", params
	default:
		return "unknown:" + strings.TrimSpace(name), params
	}
}

// parseContentParam разбирает контентный параметр нового формата
// (<param name="question" type="content"><item .../>) в последовательность атомов.
// Параметр без вложенных <item> трактуется как один текстовый фрагмент.
func parseContentParam(xp xmlParam) []model.Atom {
	var wrapped xmlItems
	if err := xml.Unmarshal([]byte("<root>"+xp.Inner+"</root>"), &wrapped); err == nil && len(wrapped.Items) > 0 {
		out := make([]model.Atom, 0, len(wrapped.Items))
		for _, it := range wrapped.Items {
			text := strings.TrimSpace(it.Text)
			if text == "" {
				continue
			}
			atom := model.Atom{Kind: normalizeAtomKind(it.Type), Text: text, Time: it.Time}
			// Вложенные по ссылке файлы — всегда медиа, даже без type.
			if strings.EqualFold(it.IsRef, "true") && atom.Kind == "text" {
				atom.Kind = guessMediaKind(text)
			}
			out = append(out, atom)
		}
		return out
	}
	if text := strings.TrimSpace(stripTags(xp.Inner)); text != "" {
		return []model.Atom{{Kind: "text", Text: text}}
	}
	return nil
}

// guessMediaKind определяет тип медиа по расширению файла.
func guessMediaKind(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg":
		return "image"
	case ".mp3", ".ogg", ".wav", ".flac", ".m4a", ".aac":
		return "audio"
	case ".mp4", ".webm", ".avi", ".mkv", ".mov", ".m4v":
		return "video"
	default:
		return "text"
	}
}

// materializeAtom резолвит медиа-ссылки атома и отсекает служебные виды.
// Возвращает false, если атом нужно пропустить (маркер или битая ссылка).
func materializeAtom(atom *model.Atom, extractor *mediaExtractor, res *Result, qid string) bool {
	if atom.Kind == "marker" {
		return false
	}
	if atom.Kind == "image" || atom.Kind == "audio" || atom.Kind == "video" {
		u, warn := extractor.Resolve(atom.Text)
		if u == "" {
			res.Warnings = append(res.Warnings, fmt.Sprintf("вопрос %s: %s", qid, warn))
			return false
		}
		atom.MediaURL = u
		atom.Text = ""
	}
	return true
}

// numberSetMinimum возвращает минимум из <param name="price" type="numberSet">.
func numberSetMinimum(xq *xmlQuestion) (int64, bool) {
	for _, p := range xq.Params {
		if !strings.EqualFold(strings.TrimSpace(p.Name), "price") {
			continue
		}
		var ns xmlNumberSet
		if err := xml.Unmarshal([]byte(p.Inner), &ns); err != nil {
			// Внутренность может быть обёрнута неполным тегом — пробуем как есть.
			continue
		}
		if ns.Minimum > 0 {
			return int64(ns.Minimum + 0.5), true
		}
	}
	return 0, false
}

// stripTags удаляет XML-теги из фрагмента (для текстовых параметров).
func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func normalizeAtomKind(k string) string {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case "", "text":
		return "text"
	case "image":
		return "image"
	case "voice", "audio":
		return "audio"
	case "video":
		return "video"
	case "say":
		return "say"
	case "marker":
		return "marker"
	default:
		return "text"
	}
}

func cleanList(in []string) []string {
	out := in[:0]
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := in[:0]
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// --- Извлечение медиа ---

type mediaExtractor struct {
	byName  map[string]*zip.File
	destDir string
	urlBase string
	used    map[string]string // lower(clean zip path) -> public url
}

// Resolve находит файл по ссылке вида "@name.png" / "Images/name.png",
// извлекает его и возвращает публичный URL. Второе значение — текст предупреждения.
func (m *mediaExtractor) Resolve(ref string) (string, string) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", ""
	}
	ref = strings.TrimPrefix(ref, "@")
	if dec, err := url.PathUnescape(ref); err == nil {
		ref = dec
	}
	ref = strings.ReplaceAll(ref, "\\", "/")
	clean := strings.ToLower(path.Clean(ref))
	if strings.HasPrefix(clean, "..") || strings.HasPrefix(clean, "/") {
		return "", fmt.Sprintf("небезопасная ссылка на медиа: %q", ref)
	}

	f := m.byName[clean]
	if f == nil {
		// Пробуем найти по basename (пакеты часто ссылаются без каталога).
		base := path.Base(clean)
		for name, cand := range m.byName {
			if path.Base(name) == base {
				f = cand
				break
			}
		}
	}
	if f == nil {
		return "", fmt.Sprintf("медиафайл не найден в архиве: %q", ref)
	}
	if u, ok := m.used[clean]; ok {
		return u, ""
	}

	safe := sanitizeFilename(path.Base(f.Name))
	// Уникальность имени в каталоге назначения.
	dst := filepath.Join(m.destDir, safe)
	for i := 1; ; i++ {
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			break
		}
		ext := filepath.Ext(safe)
		dst = filepath.Join(m.destDir, fmt.Sprintf("%s_%d%s", strings.TrimSuffix(safe, ext), i, ext))
	}

	src, err := f.Open()
	if err != nil {
		return "", fmt.Sprintf("не удалось открыть медиафайл %q: %v", ref, err)
	}
	defer src.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Sprintf("не удалось сохранить медиафайл: %v", err)
	}
	if _, err := io.Copy(out, io.LimitReader(src, maxEntryUncompressed)); err != nil {
		out.Close()
		return "", fmt.Sprintf("ошибка записи медиафайла: %v", err)
	}
	out.Close()

	publicURL := m.urlBase + filepath.Base(dst)
	m.used[clean] = publicURL
	return publicURL, ""
}

func sanitizeFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	s := b.String()
	if s == "" || s == "." {
		s = "file"
	}
	if len(s) > 128 {
		ext := filepath.Ext(s)
		s = s[:128-len(ext)] + ext
	}
	return s
}
