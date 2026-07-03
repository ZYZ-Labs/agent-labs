import java.util.ArrayList;
import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public class Main {

    private static final String SAMPLE_CODE = """
            \"\"\"A tiny module for demonstration.\"\"\"

            import json
            from datetime import datetime


            def greet(name: str) -> str:
                \"\"\"Return a friendly greeting.\"\"\"
                return f"Hello, {name}!"


            class Calculator:
                \"\"\"Simple calculator.\"\"\"

                def add(self, a: float, b: float) -> float:
                    return a + b

                def subtract(self, a: float, b: float) -> float:
                    return a - b


            def main():
                calc = Calculator()
                print(greet("world"))
                print(calc.add(1, 2))
            """;

    private static final Pattern DEFINITION_PATTERN = Pattern.compile(
            "^\\s*(?:def|class)\\s+([A-Za-z0-9_]+)", Pattern.MULTILINE);

    public static void main(String[] args) {
        String source = SAMPLE_CODE;
        byte[] sourceBytes = source.getBytes(java.nio.charset.StandardCharsets.UTF_8);

        int nodeCount = countApproximateNodes(source);
        System.out.println("AST node count: " + nodeCount);

        List<Definition> definitions = extractDefinitions(source, sourceBytes);
        System.out.println("\nDiscovered definitions:");
        for (Definition d : definitions) {
            System.out.printf("  [%s] %s @ line %d%n", d.kind, d.name, d.line);
        }

        String context = buildPromptContext(source, definitions);
        System.out.println("\n" + "=".repeat(50));
        System.out.println("Generated prompt context");
        System.out.println("=".repeat(50));
        System.out.println(context);
    }

    private static int countApproximateNodes(String source) {
        // Approximate AST node count by counting significant tokens.
        int count = 0;
        Matcher matcher = Pattern.compile("\\w+|[^\\w\\s]").matcher(source);
        while (matcher.find()) {
            count++;
        }
        return count;
    }

    private static List<Definition> extractDefinitions(String source, byte[] sourceBytes) {
        List<Definition> definitions = new ArrayList<>();
        Matcher matcher = DEFINITION_PATTERN.matcher(source);
        while (matcher.find()) {
            String kind = source.substring(matcher.start(1) - 4, matcher.start(1) - 1).trim();
            if ("def".equals(kind)) {
                kind = "function";
            } else if ("class".equals(kind)) {
                kind = "class";
            }
            String name = matcher.group(1);
            int line = source.substring(0, matcher.start(1)).split("\n", -1).length;
            int startByte = source.substring(0, matcher.start()).getBytes(java.nio.charset.StandardCharsets.UTF_8).length;
            int endByte = sourceBytes.length;
            definitions.add(new Definition(kind, name, line, startByte, endByte));
        }
        return definitions;
    }

    private static String buildPromptContext(String source, List<Definition> definitions) {
        List<String> lines = new ArrayList<>();
        lines.add("You are reviewing the following Python module.");
        lines.add("");
        lines.add("## Symbols");
        for (Definition d : definitions) {
            String kind = d.kind.substring(0, 1).toUpperCase() + d.kind.substring(1);
            lines.add(String.format("- %s `%s` at line %d (bytes %d-%d)",
                    kind, d.name, d.line, d.startByte, d.endByte));
        }
        lines.add("");
        lines.add("## Source");
        lines.add("```python");
        lines.add(source);
        lines.add("```");
        lines.add("");
        lines.add("Please summarize what this module does.");
        return String.join("\n", lines);
    }

    private static class Definition {
        final String kind;
        final String name;
        final int line;
        final int startByte;
        final int endByte;

        Definition(String kind, String name, int line, int startByte, int endByte) {
            this.kind = kind;
            this.name = name;
            this.line = line;
            this.startByte = startByte;
            this.endByte = endByte;
        }
    }
}
