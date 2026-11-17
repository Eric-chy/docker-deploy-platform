import type { Diagnostic } from "@codemirror/lint";
import {
  isMap,
  parseDocument,
  type Node,
  type ParsedNode,
  type YAMLMap
} from "yaml";
import { translate as tt } from "@/plugins/i18n";

function nodeRange(
  node: Node | null,
  documentLength: number
): { from: number; to: number } {
  if (!node?.range) return { from: 0, to: documentLength };
  const [start, valueEnd, nodeEnd] = node.range;
  return {
    from: Math.max(0, start),
    to: Math.min(documentLength, nodeEnd || valueEnd)
  };
}

function scalarText(node: ParsedNode | null): string | undefined {
  return node && node.toJSON() === undefined
    ? undefined
    : String(node.toJSON());
}

function isYamlMap(node: Node | null): boolean {
  return isMap(node);
}

export function validateComposeYaml(value: string): Diagnostic[] {
  if (!value.trim()) return [];

  const document = parseDocument(value, { prettyErrors: false });
  const diagnostics: Diagnostic[] = [];
  const documentLength = value.length;

  for (const error of document.errors) {
    diagnostics.push({
      from: Math.max(0, error.pos[0]),
      to: Math.min(documentLength, Math.max(error.pos[0], error.pos[1])),
      severity: "error",
      source: "yaml",
      message: error.message
    });
  }
  for (const warning of document.warnings) {
    diagnostics.push({
      from: Math.max(0, warning.pos[0]),
      to: Math.min(documentLength, Math.max(warning.pos[0], warning.pos[1])),
      severity: "warning",
      source: "yaml",
      message: warning.message
    });
  }
  if (document.errors.length > 0 || document.warnings.length > 0)
    return diagnostics;

  const root = document.contents;
  const rootMap = isYamlMap(root) ? (root as YAMLMap.Parsed) : null;
  if (!rootMap) {
    diagnostics.push({
      ...nodeRange(root, documentLength),
      severity: "error",
      source: "compose",
      message: tt("Compose 文件必须是一个 YAML 对象")
    });
    return diagnostics;
  }

  const servicesPair = rootMap.items.find(
    item => scalarText(item.key as ParsedNode) === "services"
  );
  if (!servicesPair) {
    diagnostics.push({
      from: 0,
      to: documentLength,
      severity: "error",
      source: "compose",
      message: tt("缺少 services 字段")
    });
    return diagnostics;
  }

  const services = servicesPair.value;
  const servicesMap = isYamlMap(services) ? (services as YAMLMap.Parsed) : null;
  if (!servicesMap || servicesMap.items.length === 0) {
    diagnostics.push({
      ...nodeRange(services, documentLength),
      severity: "error",
      source: "compose",
      message: tt("services 必须是非空对象")
    });
    return diagnostics;
  }

  for (const servicePair of servicesMap.items) {
    const serviceName = scalarText(servicePair.key as ParsedNode) ?? "";
    const service = servicePair.value;
    const serviceRange = nodeRange(service, documentLength);
    const serviceMap = isYamlMap(service) ? (service as YAMLMap.Parsed) : null;
    if (!serviceMap) {
      diagnostics.push({
        ...serviceRange,
        severity: "error",
        source: "compose",
        message: tt("服务 {name} 必须是一个对象", { name: serviceName })
      });
      continue;
    }

    const image = serviceMap.items.find(
      item => scalarText(item.key as ParsedNode) === "image"
    );
    const build = serviceMap.items.find(
      item => scalarText(item.key as ParsedNode) === "build"
    );
    if (
      !image ||
      image.value?.toJSON() === undefined ||
      String(image.value.toJSON()).trim() === ""
    ) {
      if (!build || build.value?.toJSON() === undefined) {
        diagnostics.push({
          ...serviceRange,
          severity: "error",
          source: "compose",
          message: tt("服务 {name} 必须配置 image 或 build", {
            name: serviceName
          })
        });
      }
    }

    const ports = serviceMap.items.find(
      item => scalarText(item.key as ParsedNode) === "ports"
    );
    if (ports && !Array.isArray(ports.value?.toJSON())) {
      diagnostics.push({
        ...nodeRange(ports.value, documentLength),
        severity: "error",
        source: "compose",
        message: tt("服务 {name} 的 ports 必须是数组", { name: serviceName })
      });
    }
  }

  return diagnostics;
}
