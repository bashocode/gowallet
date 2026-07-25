package security_test

import (
	"strings"
	"testing"

	"github.com/bashocode/gowallet/microservices/shared/security"
)

func TestSanitizeString_StripsScriptTag(t *testing.T) {
	s := security.NewSanitizer()

	input := `<script>alert('xss')</script>Hello`
	want := "Hello"

	got := s.SanitizeString(input)
	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestSanitizeString_StripsImgOnerror(t *testing.T) {
	s := security.NewSanitizer()

	input := `<img src=x onerror="alert(1)">`
	want := ""

	got := s.SanitizeString(input)
	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestSanitizeString_StripsIframe(t *testing.T) {
	s := security.NewSanitizer()

	input := `<iframe src="https://evil.com"></iframe>safe`
	want := "safe"

	got := s.SanitizeString(input)
	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestSanitizeString_PreservesNormalText(t *testing.T) {
	s := security.NewSanitizer()

	input := "Budi Santoso"
	want := "Budi Santoso"

	got := s.SanitizeString(input)
	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestSanitizeString_StripsSVGOnload(t *testing.T) {
	s := security.NewSanitizer()

	input := `<svg onload="alert(1)">`
	want := ""

	got := s.SanitizeString(input)
	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestTruncateAndSanitize(t *testing.T) {
	s := security.NewSanitizer()

	input := `<script>evil()</script>` + strings.Repeat("A", 200)
	got := s.TruncateAndSanitize(input, 100)

	if len(got) > 100 {
		t.Errorf("Result length = %d, want <= 100", len(got))
	}
	if strings.Contains(got, "<") {
		t.Errorf("Result contains HTML: %q", got)
	}
}

func TestSanitizeString_StripsMultipleScriptTags(t *testing.T) {
	s := security.NewSanitizer()

	input := `<script>alert(1)</script>Hello<script>alert(2)</script>World`
	want := "HelloWorld"

	got := s.SanitizeString(input)
	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestSanitizeString_StripsOnclickHandler(t *testing.T) {
	s := security.NewSanitizer()

	input := `<div onclick="alert('xss')">Click me</div>`
	want := "Click me"

	got := s.SanitizeString(input)
	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestSanitizeString_StripsOnmouseoverHandler(t *testing.T) {
	s := security.NewSanitizer()

	input := `<span onmouseover="alert(1)">Hover</span>`
	want := "Hover"

	got := s.SanitizeString(input)
	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestSanitizeString_StripsNestedHTML(t *testing.T) {
	s := security.NewSanitizer()

	input := `<div><span><b>Bold</b></span></div>`
	want := "Bold"

	got := s.SanitizeString(input)
	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestSanitizeString_HandlesEmptyString(t *testing.T) {
	s := security.NewSanitizer()

	input := ""
	want := ""

	got := s.SanitizeString(input)
	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestSanitizeString_StripsStyleTag(t *testing.T) {
	s := security.NewSanitizer()

	input := `<style>body{background:red;}</style>Content`
	want := "Content"

	got := s.SanitizeString(input)
	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestSanitizeString_StripsAnchorTag(t *testing.T) {
	s := security.NewSanitizer()

	input := `<a href="javascript:alert(1)">Click</a>`
	want := "Click"

	got := s.SanitizeString(input)
	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestSanitizeString_HandlesSpecialCharacters(t *testing.T) {
	s := security.NewSanitizer()

	input := "Hello & goodbye < > \" '"
	want := "Hello &amp; goodbye &lt; &gt; &#34; &#39;"

	got := s.SanitizeString(input)

	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestSanitizeString_StripsJavascriptProtocol(t *testing.T) {
	s := security.NewSanitizer()

	input := `<a href="javascript:alert('xss')">Click Here</a>`
	want := "Click Here"

	got := s.SanitizeString(input)
	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestSanitizeString_StripsMalformedNestedScript(t *testing.T) {
	s := security.NewSanitizer()

	input := `<scr<script>ipt>alert(1)</script>`
	want := "ipt&gt;alert(1)"

	got := s.SanitizeString(input)
	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestSanitizeMap_SanitizesStringValues(t *testing.T) {
	s := security.NewSanitizer()

	m := map[string]interface{}{
		"name":  "<script>alert(1)</script>John",
		"email": "john@example.com",
		"bio":   "<b>Developer</b>",
	}

	s.SanitizeMap(m)

	if name, ok := m["name"].(string); ok {
		if strings.Contains(name, "<script>") {
			t.Errorf("Map still contains script tag: %q", name)
		}
		if !strings.Contains(name, "John") {
			t.Errorf("Map should preserve text content: %q", name)
		}
	}

	if bio, ok := m["bio"].(string); ok {
		if strings.Contains(bio, "<b>") {
			t.Errorf("Map still contains HTML tag: %q", bio)
		}
		if !strings.Contains(bio, "Developer") {
			t.Errorf("Map should preserve text content: %q", bio)
		}
	}
}

func TestSanitizeMap_HandlesNestedMaps(t *testing.T) {
	s := security.NewSanitizer()

	m := map[string]interface{}{
		"user": map[string]interface{}{
			"name": "<script>alert(1)</script>Jane",
			"bio":  "<img src=x onerror=alert(1)>",
		},
	}

	s.SanitizeMap(m)

	if user, ok := m["user"].(map[string]interface{}); ok {
		if name, ok := user["name"].(string); ok {
			if strings.Contains(name, "<script>") {
				t.Errorf("Nested map still contains script tag: %q", name)
			}
		}
	}
}

func TestSanitizeSlice_SanitizesStringValues(t *testing.T) {
	s := security.NewSanitizer()

	arr := []interface{}{
		"<script>alert(1)</script>Hello",
		"Normal text",
		"<b>Bold</b>",
	}

	s.SanitizeSlice(arr)

	if val, ok := arr[0].(string); ok {
		if strings.Contains(val, "<script>") {
			t.Errorf("Slice still contains script tag: %q", val)
		}
	}

	if val, ok := arr[2].(string); ok {
		if strings.Contains(val, "<b>") {
			t.Errorf("Slice still contains HTML tag: %q", val)
		}
	}
}

func TestSanitizeStruct_SanitizesAllFields(t *testing.T) {
	s := security.NewSanitizer()

	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Bio   string `json:"bio"`
	}

	user := User{
		Name:  "<script>alert(1)</script>Alice",
		Email: "alice@example.com",
		Bio:   "<img src=x onerror=alert(1)>Developer",
	}

	err := s.SanitizeStruct(&user)
	if err != nil {
		t.Fatalf("SanitizeStruct() error = %v", err)
	}

	if strings.Contains(user.Name, "<script>") {
		t.Errorf("Struct still contains script tag: %q", user.Name)
	}

	if strings.Contains(user.Bio, "<img") {
		t.Errorf("Struct still contains img tag: %q", user.Bio)
	}

	if !strings.Contains(user.Name, "Alice") {
		t.Errorf("Struct should preserve text content: %q", user.Name)
	}
}

func TestSanitizeString_StripsDataAttribute(t *testing.T) {
	s := security.NewSanitizer()

	input := `<div data-payload="<script>alert(1)</script>">Content</div>`
	want := "Content"

	got := s.SanitizeString(input)
	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestSanitizeString_StripsObjectTag(t *testing.T) {
	s := security.NewSanitizer()

	input := `<object data="javascript:alert(1)"></object>Text`
	want := "Text"

	got := s.SanitizeString(input)
	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestSanitizeString_StripsEmbedTag(t *testing.T) {
	s := security.NewSanitizer()

	input := `<embed src="javascript:alert(1)">Text`
	want := "Text"

	got := s.SanitizeString(input)
	if got != want {
		t.Errorf("SanitizeString() = %q, want %q", got, want)
	}
}

func TestTruncateAndSanitize_PreservesTextWithinLimit(t *testing.T) {
	s := security.NewSanitizer()

	input := "Hello World"
	got := s.TruncateAndSanitize(input, 100)

	if got != "Hello World" {
		t.Errorf("TruncateAndSanitize() = %q, want %q", got, "Hello World")
	}
}

func TestTruncateAndSanitize_TrimsWhitespace(t *testing.T) {
	s := security.NewSanitizer()

	input := "  Hello World  "
	got := s.TruncateAndSanitize(input, 100)

	if got != "Hello World" {
		t.Errorf("TruncateAndSanitize() = %q, want %q", got, "Hello World")
	}
}
