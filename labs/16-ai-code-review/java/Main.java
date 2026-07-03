import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import java.util.Map;

public class Main {

    private static final String REVIEW_PROMPT = """
            You are a meticulous code reviewer. Review the following Python source file and return ONLY a JSON object.

            Rules for the JSON object:
            - Top-level keys must be exactly: "security", "style", "logic".
            - Each value is a list of findings. An empty list is allowed.
            - Each finding is an object with these keys:
              - "severity": one of "HIGH", "MEDIUM", "LOW".
              - "line": integer line number, or null if not applicable.
              - "message": concise description of the issue.
              - "suggestion": concrete recommendation to fix it.

            Be strict but fair. Focus on real issues, not nitpicks.

            ```python
            {code}
            ```
            """;

    private static final List<String> CATEGORIES = List.of("security", "style", "logic");
    private static final Map<String, Integer> SEVERITY_ORDER = Map.of("HIGH", 0, "MEDIUM", 1, "LOW", 2);

    public static void main(String[] args) throws Exception {
        OpenAiClient client = new OpenAiClient();
        ObjectMapper mapper = new ObjectMapper();

        Path target = args.length > 0 ? Paths.get(args[0]) : Paths.get("..", "python", "sample_code.py");
        if (!Files.exists(target) || !Files.isRegularFile(target)) {
            System.err.println("Source file not found: " + target);
            System.exit(1);
        }

        String source = Files.readString(target);
        Map<String, Object> report = reviewCode(client, mapper, source);
        validateReport(report);
        printReport(target, source.split("\\n", -1).length, report);
    }

    private static Map<String, Object> reviewCode(OpenAiClient client, ObjectMapper mapper, String source) throws Exception {
        String prompt = REVIEW_PROMPT.replace("{code}", source);
        System.out.println("Sending code review prompt (" + source.length() + " chars of code).");

        ObjectNode payload = mapper.createObjectNode();
        ArrayNode messages = mapper.createArrayNode();
        ObjectNode user = messages.addObject();
        user.put("role", "user");
        user.put("content", prompt);
        payload.set("messages", messages);
        payload.put("temperature", 0.2);
        payload.put("max_tokens", 1200);
        ObjectNode responseFormat = mapper.createObjectNode();
        responseFormat.put("type", "json_object");
        payload.set("response_format", responseFormat);

        JsonNode response = client.chatCompletion(payload);
        String content = client.extractMessage(response).get("content").asText();
        try {
            return mapper.readValue(content, Map.class);
        } catch (Exception exc) {
            throw new IllegalArgumentException("Model returned invalid JSON: " + exc.getMessage() + "\nRaw content:\n" + content, exc);
        }
    }

    private static void validateReport(Map<String, Object> report) {
        if (!(report instanceof Map)) {
            throw new IllegalArgumentException("Review report is not a JSON object.");
        }
        List<String> missing = new ArrayList<>();
        for (String cat : CATEGORIES) {
            if (!report.containsKey(cat)) {
                missing.add(cat);
            }
        }
        if (!missing.isEmpty()) {
            throw new IllegalArgumentException("Review report missing required categories: " + missing);
        }
        for (String category : CATEGORIES) {
            Object raw = report.get(category);
            if (!(raw instanceof List)) {
                throw new IllegalArgumentException("Category '" + category + "' must be a list.");
            }
            List<?> findings = (List<?>) raw;
            for (int idx = 0; idx < findings.size(); idx++) {
                Object fobj = findings.get(idx);
                if (!(fobj instanceof Map)) {
                    throw new IllegalArgumentException("Finding " + idx + " in '" + category + "' is not an object.");
                }
                Map<?, ?> finding = (Map<?, ?>) fobj;
                for (String key : new String[]{"severity", "message", "suggestion"}) {
                    if (!finding.containsKey(key)) {
                        throw new IllegalArgumentException("Finding " + idx + " in '" + category + "' missing key '" + key + "'.");
                    }
                }
                String severity = String.valueOf(finding.get("severity"));
                if (!SEVERITY_ORDER.containsKey(severity)) {
                    throw new IllegalArgumentException("Finding " + idx + " in '" + category + "' has invalid severity '" + severity + "'.");
                }
            }
        }
    }

    private static void printReport(Path filePath, int lineCount, Map<String, Object> report) {
        List<String> categoriesPresent = new ArrayList<>();
        for (String cat : CATEGORIES) {
            Object raw = report.get(cat);
            if (raw instanceof List && !((List<?>) raw).isEmpty()) {
                categoriesPresent.add(cat);
            }
        }

        System.out.println("\nReview: " + filePath);
        System.out.println("Lines: " + lineCount);
        System.out.println("Categories: " + (categoriesPresent.isEmpty() ? "none with findings" : String.join(", ", categoriesPresent)));

        int total = 0;
        int high = 0, medium = 0, low = 0;

        for (String category : CATEGORIES) {
            Object raw = report.get(category);
            if (!(raw instanceof List)) {
                continue;
            }
            List<Map<String, Object>> findings = new ArrayList<>();
            for (Object item : (List<?>) raw) {
                if (item instanceof Map) {
                    findings.add((Map<String, Object>) item);
                }
            }
            if (findings.isEmpty()) {
                continue;
            }
            findings.sort(Comparator.comparingInt(f -> SEVERITY_ORDER.getOrDefault(String.valueOf(f.get("severity")), 99)));
            System.out.println("\n" + category.toUpperCase());
            for (Map<String, Object> finding : findings) {
                total++;
                String severity = String.valueOf(finding.get("severity"));
                switch (severity) {
                    case "HIGH" -> high++;
                    case "MEDIUM" -> medium++;
                    case "LOW" -> low++;
                }
                Object line = finding.get("line");
                String lineInfo = line != null ? " at line " + line : "";
                System.out.println("  - " + severity + lineInfo + ": " + finding.get("message"));
                System.out.println("    Suggestion: " + finding.get("suggestion"));
            }
        }

        System.out.println("\nSummary: " + total + " issue(s) found. " + high + " high, " + medium + " medium, " + low + " low.");
    }
}
