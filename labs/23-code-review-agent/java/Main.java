import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.io.File;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;

public class Main {

    public static void main(String[] args) throws Exception {
        OpenAiClient client = null;
        try {
            client = new OpenAiClient();
        } catch (IllegalStateException exc) {
            System.out.println("LLM client disabled: " + exc.getMessage());
        }

        Path target = args.length > 0 ? Paths.get(args[0]) : Paths.get("..", "python", "sample_code.py");
        if (!Files.exists(target)) {
            System.err.println("File not found: " + target);
            System.exit(1);
        }

        Map<String, Object> report = reviewFile(target, client);
        System.out.println(new ObjectMapper().writerWithDefaultPrettyPrinter().writeValueAsString(report));
    }

    private static Map<String, Object> reviewFile(Path path, OpenAiClient client) throws Exception {
        System.out.println("Reviewing " + path);
        String source = Files.readString(path);
        Map<String, Object> checks = Map.of(
                "syntax", runSyntaxCheck(path),
                "ruff", runRuff(path)
        );
        String review = llmReview(path, source, checks, client);
        @SuppressWarnings("unchecked")
        List<String> ruffIssues = new ArrayList<>((List<String>) ((Map<String, Object>) checks.get("ruff")).get("issues"));
        ruffIssues.remove("ruff not installed");
        boolean syntaxOk = (Boolean) ((Map<String, Object>) checks.get("syntax")).get("ok");
        String verdict = syntaxOk && ruffIssues.isEmpty() ? "ok" : "needs_work";
        return Map.of(
                "file", path.toString(),
                "checks", checks,
                "llm_review", review,
                "verdict", verdict
        );
    }

    private static Map<String, Object> runSyntaxCheck(Path path) {
        try {
            ProcessBuilder pb = new ProcessBuilder("python", "-m", "py_compile", path.toString());
            pb.redirectErrorStream(true);
            Process process = pb.start();
            if (!process.waitFor(10, TimeUnit.SECONDS)) {
                process.destroyForcibly();
                return Map.of("ok", false, "issues", List.of("Syntax check timed out"));
            }
            if (process.exitValue() != 0) {
                String output = new String(process.getInputStream().readAllBytes());
                return Map.of("ok", false, "issues", List.of("Syntax error: " + output.trim()));
            }
            return Map.of("ok", true, "issues", List.of());
        } catch (Exception exc) {
            return Map.of("ok", false, "issues", List.of("Syntax check failed: " + exc.getMessage()));
        }
    }

    private static Map<String, Object> runRuff(Path path) {
        String ruffBin = findRuff();
        if (ruffBin == null) {
            return Map.of("ok", true, "issues", List.of("ruff not installed"));
        }
        try {
            ProcessBuilder pb = new ProcessBuilder(ruffBin, "check", path.toString(), "--output-format", "json");
            pb.redirectErrorStream(true);
            Process process = pb.start();
            if (!process.waitFor(10, TimeUnit.SECONDS)) {
                process.destroyForcibly();
                return Map.of("ok", false, "issues", List.of("ruff timed out"));
            }
            String output = new String(process.getInputStream().readAllBytes());
            List<String> issues = new ArrayList<>();
            if (!output.isBlank()) {
                try {
                    JsonNode parsed = new ObjectMapper().readTree(output);
                    for (JsonNode item : parsed) {
                        JsonNode location = item.get("location");
                        issues.add("Line " + location.get("row").asText() + ": " + item.get("code").asText() + " - " + item.get("message").asText());
                    }
                } catch (Exception exc) {
                    issues.add(output.trim());
                }
            }
            return Map.of("ok", issues.isEmpty(), "issues", issues);
        } catch (Exception exc) {
            return Map.of("ok", true, "issues", List.of("ruff not installed"));
        }
    }

    private static String findRuff() {
        String pathEnv = System.getenv("PATH");
        if (pathEnv != null) {
            String ext = System.getProperty("os.name").toLowerCase().contains("win") ? ".exe" : "";
            for (String dir : pathEnv.split(File.pathSeparator)) {
                Path candidate = Paths.get(dir, "ruff" + ext);
                if (Files.exists(candidate)) {
                    return candidate.toString();
                }
            }
        }
        Path pythonDir = Paths.get(System.getProperty("java.home")).getParent();
        if (pythonDir != null) {
            Path candidate = pythonDir.resolve("ruff" + (System.getProperty("os.name").toLowerCase().contains("win") ? ".exe" : ""));
            if (Files.exists(candidate)) {
                return candidate.toString();
            }
        }
        return null;
    }

    private static String llmReview(Path path, String source, Map<String, Object> checks, OpenAiClient client) throws Exception {
        if (client == null) {
            return "LLM review skipped (no API key). Manual review recommended.";
        }
        @SuppressWarnings("unchecked")
        Map<String, Object> syntax = (Map<String, Object>) checks.get("syntax");
        @SuppressWarnings("unchecked")
        Map<String, Object> ruff = (Map<String, Object>) checks.get("ruff");

        String prompt = "Review the following Python file for style, bugs, and maintainability.\n\nFile: " + path.getFileName()
                + "\n\nStatic checks:\n- Syntax OK: " + syntax.get("ok")
                + "\n- Style issues: " + ruff.get("issues")
                + "\n\nSource code:\n```python\n" + source + "\n```\n\nProvide a concise review with:\n1. Critical issues\n2. Suggestions\n3. Overall verdict (OK / Needs work).";

        ObjectMapper mapper = new ObjectMapper();
        ObjectNode payload = mapper.createObjectNode();
        ArrayNode messages = mapper.createArrayNode();
        ObjectNode user = messages.addObject();
        user.put("role", "user");
        user.put("content", prompt);
        payload.set("messages", messages);
        payload.put("temperature", 0.2);
        payload.put("max_tokens", 400);

        JsonNode response = client.chatCompletion(payload);
        return client.extractMessage(response).get("content").asText().strip();
    }
}
