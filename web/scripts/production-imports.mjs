import ts from "typescript";

// Inspect module specifiers, not prose or data strings containing "prototype".
export function prototypeImports(source, filename) {
  const file = ts.createSourceFile(filename, source, ts.ScriptTarget.Latest, true,
    filename.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS);
  const findings = [];
  function inspect(node) {
    let specifier;
    if (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) specifier = node.moduleSpecifier;
    if (ts.isCallExpression(node) && (node.expression.kind === ts.SyntaxKind.ImportKeyword ||
      (ts.isIdentifier(node.expression) && node.expression.text === "require"))) specifier = node.arguments[0];
    if (ts.isImportTypeNode(node) && ts.isLiteralTypeNode(node.argument)) specifier = node.argument.literal;
    if (specifier && (ts.isStringLiteralLike(specifier)) && /(?:^|[/\\])prototypes?(?:[/\\]|$)/i.test(specifier.text)) {
      findings.push(`${filename}:${file.getLineAndCharacterOfPosition(node.getStart(file)).line + 1}: production prototype import`);
    }
    ts.forEachChild(node, inspect);
  }
  inspect(file);
  return findings;
}
