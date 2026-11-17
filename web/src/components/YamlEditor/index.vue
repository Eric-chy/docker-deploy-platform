<script setup lang="ts">
import { lintGutter, linter } from "@codemirror/lint";
import { oneDark } from "@codemirror/theme-one-dark";
import {
  EditorView,
  keymap,
  lineNumbers,
  highlightActiveLine
} from "@codemirror/view";
import { Compartment, EditorState } from "@codemirror/state";
import { basicSetup } from "codemirror";
import { yaml } from "@codemirror/lang-yaml";
import { useDark } from "@pureadmin/utils";
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { translate as tt } from "@/plugins/i18n";
import { validateComposeYaml } from "@/utils/compose-yaml";

defineOptions({ name: "YamlEditor" });

const props = withDefaults(
  defineProps<{
    modelValue: string;
    height?: string;
  }>(),
  { height: "360px" }
);

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const editorHost = ref<HTMLDivElement>();
const { isDark } = useDark();
let view: EditorView | undefined;
const themeCompartment = new Compartment();
const diagnostics = computed(() => validateComposeYaml(props.modelValue));
const diagnosticLine = (position: number) =>
  props.modelValue.slice(0, position).split("\n").length;

const editorTheme = EditorView.theme({
  "&": { height: props.height, fontSize: "13px" },
  ".cm-scroller": { fontFamily: "var(--font-family-code, monospace)" }
});

onMounted(() => {
  if (!editorHost.value) return;
  view = new EditorView({
    state: EditorState.create({
      doc: props.modelValue,
      extensions: [
        basicSetup,
        lineNumbers(),
        highlightActiveLine(),
        yaml(),
        lintGutter(),
        linter(view => validateComposeYaml(view.state.doc.toString()), {
          delay: 200
        }),
        themeCompartment.of(isDark.value ? oneDark : []),
        editorTheme,
        EditorView.lineWrapping,
        EditorView.updateListener.of(update => {
          if (update.docChanged)
            emit("update:modelValue", update.state.doc.toString());
        })
      ]
    })
  });
  editorHost.value.appendChild(view.dom);
});

watch(
  () => props.modelValue,
  value => {
    if (!view || value === view.state.doc.toString()) return;
    view.dispatch({
      changes: { from: 0, to: view.state.doc.length, insert: value }
    });
  }
);

watch(
  isDark,
  dark => {
    if (!view) return;
    view.dispatch({
      effects: themeCompartment.reconfigure(dark ? oneDark : [])
    });
  },
  { immediate: true }
);

onUnmounted(() => {
  view?.destroy();
  view = undefined;
});
</script>

<template>
  <div
    class="overflow-hidden rounded border border-gray-300 dark:border-gray-700"
  >
    <div ref="editorHost" />
  </div>
  <div v-if="diagnostics.length" class="mt-2 space-y-1">
    <div
      v-for="(diagnostic, index) in diagnostics"
      :key="index"
      class="rounded px-2 py-1 text-xs"
      :class="
        diagnostic.severity === 'error'
          ? 'bg-red-50 text-red-600 dark:bg-red-950 dark:text-red-400'
          : 'bg-orange-50 text-orange-600 dark:bg-orange-950 dark:text-orange-400'
      "
    >
      {{ $tt("第 {line} 行", { line: diagnosticLine(diagnostic.from) }) }}：{{
        diagnostic.message
      }}
    </div>
  </div>
</template>
