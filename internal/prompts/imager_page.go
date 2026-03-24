package prompts

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shouni/go-manga-kit/ports"
)

// --- Constants & Types ---
const (
	LabelStandard     = "Standard"
	LabelFullPage     = "FULL-PAGE"
	LabelImpact       = "FULL-WIDTH IMPACT"
	PosFullPage       = "Entire page area"
	PosFullWidth      = "Bottom row, covering the entire width of the page"
	CompositionImpact = "- COMPOSITION: Cinematic wide shot, high impact focus.\n"

	// MangaStructureHeader は漫画の構造に関する基本ルールを定義します。
	MangaStructureHeader = `### FORMAT RULES: FULL COLOR ANIME MANGA ###
- STYLE: Vibrant Full Color Digital Anime Style. High saturation, cinematic lighting.
- RENDERING: Sharp clean lineart with professional digital coloring. NO screentones.
- LAYOUT: Strict multi-panel composition. Use ONLY the specified number of panels.
- NO FILLER: Do not add extra panels or decorative small frames. Fill the page with the given count.
- BORDERS: Deep black, crisp frame borders for EVERY panel.
- GUTTERS: Pure white space between panels.
- READING FLOW: Right-to-Left, Top-to-Bottom.`
)

// BuildPage はメインのプロンプト構築フローを管理します。
func (pb *ImageBuilder) BuildPage(panels []ports.Panel, rm *ports.ResourceMap) (string, string) {
	numPanels := len(panels)
	bigPanelIdx := pb.calculateBigPanelIndex(numPanels)

	// 1. システムプロンプトの構築
	systemPrompt := pb.buildSystemPrompt()

	// 2. ユーザープロンプトの構築
	var us strings.Builder
	pb.writeBasicRequirements(&us, numPanels)
	pb.writeLayoutStructure(&us, numPanels)
	pb.writeCharacterReferences(&us, rm)
	pb.writePanelBreakdown(&us, panels, rm, bigPanelIdx)

	return us.String(), systemPrompt
}

// buildSystemPrompt 一貫性を保つために、定義済みの指示、スタイル、タグを組み込んだシステム プロンプト文字列を構築します。
func (pb *ImageBuilder) buildSystemPrompt() string {
	const instr = "You are a master digital artist. You MUST follow the exact panel count and layout rules. Character identity MUST match the character master reference files."
	parts := []string{instr, MangaStructureHeader, RenderingStyle, CinematicTags}
	if pb.defaultSuffix != "" {
		parts = append(parts, fmt.Sprintf("### ARTISTIC STYLE ###\n%s", pb.defaultSuffix))
	}
	return strings.Join(parts, "\n\n")
}

// writeBasicRequirements フォーマットされた基本要件セクションを生成し、提供された文字列ビルダーに追加します。
func (pb *ImageBuilder) writeBasicRequirements(w *strings.Builder, num int) {
	w.WriteString("# FULL COLOR PAGE PRODUCTION REQUEST\n")
	w.WriteString("- OUTPUT: ONE single portrait manga page image.\n")
	w.WriteString("- COLOR: STRICTLY VIBRANT FULL COLOR. NO monochrome, NO screentones.\n")
	fmt.Fprintf(w, "- PANEL COUNT: [ %d ] (STRICTLY ONLY %d PANELS. DO NOT ADD ANY MORE).\n\n", num, num)
}

// writeLayoutStructure フォーマットされたレイアウト構造を生成し、提供された文字列ビルダーに追加します。
func (pb *ImageBuilder) writeLayoutStructure(w *strings.Builder, num int) {
	w.WriteString("## MANDATORY PAGE STRUCTURE\n")
	w.WriteString("- READING ORDER: Japanese Style (Right-to-Left, then Top-to-Bottom).\n")
	w.WriteString("- PANEL PLACEMENT MAP:\n")

	if num == 1 {
		w.WriteString("  * PANEL 1: SINGLE FULL-PAGE PANEL (covers entire image area).\n")
	} else {
		for i := 0; i < num; i++ {
			if num%2 == 1 && i == num-1 {
				fmt.Fprintf(w, "  * PANEL %d: BOTTOM ROW, FULL-WIDTH.\n", i+1)
			} else {
				row, side := (i/2)+1, "RIGHT"
				if i%2 == 1 {
					side = "LEFT"
				}
				fmt.Fprintf(w, "  * PANEL %d: ROW %d, %s column.\n", i+1, row, side)
			}
		}
	}
	w.WriteString("- FRAME STYLE: Deep black borders. GUTTERS: Pure white.\n\n")
}

// writeCharacterReferences フォーマットされた文字参照のリストを生成し、提供された文字列ビルダーに追加します。
func (pb *ImageBuilder) writeCharacterReferences(w *strings.Builder, rm *ports.ResourceMap) {
	w.WriteString("## CHARACTER MASTER REFERENCES\n")

	type charRef struct {
		id  string
		idx int
	}
	// パフォーマンス改善: キャパシティを事前に確保
	refs := make([]charRef, 0, len(rm.CharacterFiles))
	for id, idx := range rm.CharacterFiles {
		refs = append(refs, charRef{id, idx})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].idx < refs[j].idx })

	for _, r := range refs {
		name, cues := r.id, "vivid anime color palette"
		if char := pb.characterMap.GetCharacter(r.id); char != nil {
			name = char.Name
			if len(char.VisualCues) > 0 {
				cues = strings.Join(char.VisualCues, ", ")
			}
		}
		fmt.Fprintf(w, "- SUBJECT [%s]: Match input_file_%d. Traits: {%s}.\n", name, r.idx, cues)
	}
	w.WriteString("\n")
}

// writePanelBreakdown 個々のパネルのフォーマットされた内訳を生成し、提供された文字列ビルダーに追加します。
func (pb *ImageBuilder) writePanelBreakdown(w *strings.Builder, panels []ports.Panel, rm *ports.ResourceMap, bigIdx int) {
	num := len(panels)
	w.WriteString("## PANEL BREAKDOWN\n")
	for i, panel := range panels {
		panelNum := i + 1

		// メタデータ準備
		var label, pos, extraInstruction string
		if i == bigIdx {
			label, pos = (map[bool]string{true: LabelFullPage, false: LabelImpact})[num == 1], (map[bool]string{true: PosFullPage, false: PosFullWidth})[num == 1]
			if num > 1 {
				extraInstruction = CompositionImpact
			}
		} else {
			label = LabelStandard
			side := "RIGHT"
			if i%2 == 1 {
				side = "LEFT"
			}
			pos = fmt.Sprintf("Row %d, %s column", (i/2)+1, side)
		}

		// 出力処理 (構造を維持するために順序を制御)
		fmt.Fprintf(w, "### PANEL %d [%s]\n", panelNum, label)
		fmt.Fprintf(w, "- POSITION: %s\n", pos)
		if extraInstruction != "" {
			w.WriteString(extraInstruction)
		}

		// キャラクターIDと表示名の取得
		displayName := panel.SpeakerID
		charFileIdx := -1
		if char := pb.characterMap.GetCharacter(panel.SpeakerID); char != nil {
			displayName = char.Name
			if idx, ok := rm.CharacterFiles[char.ID]; ok {
				charFileIdx = idx
			}
		}

		if charFileIdx != -1 {
			fmt.Fprintf(w, "- CHARACTER_IDENTITY: [ %s ] from input_file_%d. (Face, hair, and outfit MUST match input_file_%d exactly).\n", displayName, charFileIdx, charFileIdx)
		} else {
			fmt.Fprintf(w, "- SUBJECT: %s\n", displayName)
		}

		// アクションとポーズ参照
		sceneDescription := sanitizeInline(panel.VisualAnchor)
		sceneDescription = strings.ReplaceAll(sceneDescription, panel.SpeakerID, displayName)
		fmt.Fprintf(w, "- ACTION: %s\n", sceneDescription)

		if panel.ReferenceURL != "" {
			if fileIdx, ok := rm.PanelFiles[panel.ReferenceURL]; ok {
				fmt.Fprintf(w, "- POSE_GUIDE: Use body posture from input_file_%d. IGNORE the character in it.\n", fileIdx)
				if charFileIdx != -1 {
					fmt.Fprintf(w, "- MANDATORY: Face and hair MUST be %s from input_file_%d.\n", displayName, charFileIdx)
				}
			}
		}

		// セリフ指示
		if panel.Dialogue != "" {
			fmt.Fprintf(w, "- SPEECH: Speech bubble for [%s].\n", displayName)
			fmt.Fprintf(w, "  - TEXT_TO_RENDER: \"%s\"\n", formatDialogue(panel.Dialogue))

			direction := "Vertical (Tategaki)"
			layoutDesc := "traditional Japanese manga style layout"

			// 10文字以下の短いセリフや、感嘆符(!?)が多い場合は横書きも検討する指示
			if len([]rune(panel.Dialogue)) <= 10 && strings.ContainsAny(panel.Dialogue, "!?！？") {
				// 短い叫びなどはインパクト重視で「横書き」を許可する指示を混ぜる
				direction = "Horizontal (Yokogaki) or Vertical"
				layoutDesc = "bold and high impact placement"
			}

			fmt.Fprintf(w, "  - TEXT_DIRECTION: %s\n", direction)
			fmt.Fprintf(w, "  - TYPOGRAPHY: Use professional Japanese manga font (Gothic/Mincho). %s.\n", layoutDesc)
			w.WriteString("  - LANGUAGE: Japanese characters. Ensure accurate rendering of Kanji/Kana.\n")
		}
		w.WriteString("\n")
	}
}

// calculateBigPanelIndex は拡大表示するパネルのインデックスを計算して返します。
// パネル数が1の場合は0を返します。
// パネル数が1より大きく奇数の場合は、最後のパネルのインデックス (num - 1) を返します。
// それ以外の場合（0、偶数）は、拡大パネルなしを示す-1を返します。
func (pb *ImageBuilder) calculateBigPanelIndex(num int) int {
	if num == 1 {
		return 0
	}
	if num > 1 && num%2 == 1 {
		return num - 1
	}
	return -1
}
