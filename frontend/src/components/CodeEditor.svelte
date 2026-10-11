<script lang="ts">
  import { onMount } from 'svelte'
  import { EditorView, keymap, lineNumbers, highlightActiveLine, drawSelection, placeholder as ph } from '@codemirror/view'
  import { EditorState, Compartment } from '@codemirror/state'
  import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
  import { HighlightStyle, syntaxHighlighting, bracketMatching, foldGutter, indentOnInput } from '@codemirror/language'
  import { closeBrackets } from '@codemirror/autocomplete'
  import { searchKeymap, highlightSelectionMatches } from '@codemirror/search'
  import { json } from '@codemirror/lang-json'
  import { xml } from '@codemirror/lang-xml'
  import { tags } from '@lezer/highlight'

  type Props = {
    value: string | undefined
    language?: 'json' | 'xml' | 'text'
    readonly?: boolean
    placeholder?: string
    onchange?: (v: string) => void
  }
  let { value = $bindable(), language = 'text', readonly = false, placeholder = '', onchange }: Props = $props()

  let el: HTMLDivElement
  let view: EditorView | null = null
  const lang = new Compartment()
  const ro = new Compartment()

  const highlight = HighlightStyle.define([
    { tag: tags.propertyName, color: 'var(--code-key)' },
    { tag: [tags.string, tags.attributeValue], color: 'var(--code-string)' },
    { tag: [tags.number, tags.bool, tags.null], color: 'var(--code-number)' },
    { tag: [tags.keyword, tags.tagName], color: 'var(--code-keyword)' },
    { tag: tags.attributeName, color: 'var(--code-key)' },
    { tag: tags.comment, color: 'var(--code-comment)', fontStyle: 'italic' },
  ])

  const theme = EditorView.theme({
    '&': { height: '100%', fontSize: '12px', backgroundColor: 'var(--bg)', color: 'var(--text)' },
    '.cm-scroller': { fontFamily: 'var(--mono)', lineHeight: '1.5' },
    '.cm-gutters': { backgroundColor: 'var(--bg-2)', color: 'var(--text-2)', border: 'none' },
    '.cm-activeLine': { backgroundColor: 'transparent' },
    '&.cm-focused .cm-activeLine': { backgroundColor: 'var(--bg-2)' },
    '&.cm-focused': { outline: 'none' },
    '.cm-cursor': { borderLeftColor: 'var(--text)' },
    '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection': { backgroundColor: 'var(--var-ok-bg) !important' },
    '.cm-placeholder': { color: 'var(--text-2)' },
  })

  function langExt(l: string) {
    if (l === 'json') return json()
    if (l === 'xml') return xml()
    return []
  }

  onMount(() => {
    view = new EditorView({
      parent: el,
      state: EditorState.create({
        doc: value ?? '',
        extensions: [
          lineNumbers(), foldGutter(), history(), drawSelection(), indentOnInput(), bracketMatching(),
          closeBrackets(), highlightActiveLine(), highlightSelectionMatches(),
          keymap.of([...defaultKeymap, ...historyKeymap, ...searchKeymap, indentWithTab]),
          syntaxHighlighting(highlight), theme, EditorView.lineWrapping,
          ph(placeholder),
          lang.of(langExt(language)),
          ro.of(EditorState.readOnly.of(readonly)),
          EditorView.updateListener.of((u) => {
            if (u.docChanged) {
              const v = u.state.doc.toString()
              value = v
              onchange?.(v)
            }
          }),
        ],
      }),
    })
    return () => view?.destroy()
  })

  // External value changes (e.g. new response, tab switch) replace the doc.
  $effect(() => {
    const v = value ?? ''
    if (view && v !== view.state.doc.toString()) {
      view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: v } })
    }
  })
  $effect(() => {
    const l = language
    view?.dispatch({ effects: lang.reconfigure(langExt(l)) })
  })
  $effect(() => {
    const r = readonly
    view?.dispatch({ effects: ro.reconfigure(EditorState.readOnly.of(r)) })
  })
</script>

<div class="cm-host" bind:this={el}></div>

<style>
  .cm-host { height: 100%; min-height: 0; overflow: hidden; }
</style>
