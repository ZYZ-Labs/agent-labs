import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.nio.file.Files;
import java.nio.file.Paths;
import java.util.*;

public class Main {
    public static void main(String[] args) throws Exception {
        OpenAiClient client = new OpenAiClient();
        ObjectMapper mapper = new ObjectMapper();

        JsonNode cases = mapper.readTree(Files.readAllBytes(Paths.get("test_cases.json")));
        List<Map<String, Object>> results = new ArrayList<>();

        for (JsonNode c : cases) {
            long start = System.nanoTime();
            String answer = runAgent(client, c.get("input").asText());
            double latencyMs = (System.nanoTime() - start) / 1_000_000.0;

            Map<String, Object> ruleResult = ruleCheck(answer, c, latencyMs);
            Map<String, Object> judgeResult = llmJudge(client, answer, c.get("reference").asText());

            results.add(Map.of(
                "id", c.get("id").asText(),
                "input", c.get("input").asText(),
                "answer", answer,
                "rulePassed", ruleResult.get("passed"),
                "ruleDetails", ruleResult.get("checks"),
                "judgeScore", judgeResult.get("score"),
                "judgeReason", judgeResult.get("reason")
            ));
        }

        long rulePassed = results.stream().filter(r -> (Boolean) r.get("rulePassed")).count();
        double avgScore = results.stream()
            .mapToDouble(r -> ((Number) r.getOrDefault("judgeScore", 0)).doubleValue())
            .average().orElse(0.0);

        Map<String, Object> report = Map.of(
            "total", results.size(),
            "rulePassRate", results.isEmpty() ? 0.0 : (double) rulePassed / results.size(),
            "averageJudgeScore", avgScore,
            "cases", results
        );
        System.out.println(mapper.writerWithDefaultPrettyPrinter().writeValueAsString(report));
    }

    private static String runAgent(OpenAiClient client, String question) throws Exception {
        ObjectNode payload = client.buildMessages(question);
        payload.put("max_tokens", 200);
        payload.put("temperature", 0.0);
        JsonNode response = client.chatCompletion(payload);
        return client.extractMessage(response).get("content").asText();
    }

    private static Map<String, Object> ruleCheck(String answer, JsonNode testCase, double latencyMs) {
        Map<String, Object> checks = new LinkedHashMap<>();
        List<String> keywords = new ArrayList<>();
        if (testCase.has("expected_keywords")) {
            testCase.get("expected_keywords").forEach(n -> keywords.add(n.asText()));
        }
        List<String> missing = keywords.stream()
            .filter(kw -> !answer.toLowerCase().contains(kw.toLowerCase())).toList();
        checks.put("keywords", Map.of("passed", missing.isEmpty(), "missing", missing));

        if (testCase.has("expect_json") && testCase.get("expect_json").asBoolean()) {
            try {
                new ObjectMapper().readTree(answer);
                checks.put("json", Map.of("passed", true));
            } catch (Exception e) {
                checks.put("json", Map.of("passed", false, "error", e.getMessage()));
            }
        }

        if (testCase.has("max_latency_ms")) {
            double max = testCase.get("max_latency_ms").asDouble();
            checks.put("latency", Map.of("passed", latencyMs <= max, "latencyMs", latencyMs));
        }

        boolean allPassed = checks.values().stream()
            .allMap<String, Object>(v -> (Boolean) ((Map<String, Object>) v).get("passed"));
        return Map.of("passed", allPassed, "checks", checks);
    }

    private static Map<String, Object> llmJudge(OpenAiClient client, String answer, String reference) throws Exception {
        String prompt = String.format(
            "Rate how well the following answer matches the reference answer.\nAnswer: %s\nReference: %s\nRespond with JSON only: {\"score\": 1-10, \"reason\": \"...\"}",
            answer, reference
        );
        ObjectNode payload = client.buildMessages(prompt);
        payload.put("response_format", new ObjectMapper().readTree("{\"type\": \"json_object\"}"));
        payload.put("max_tokens", 200);
        payload.put("temperature", 0.0);
        try {
            JsonNode response = client.chatCompletion(payload);
            return new ObjectMapper().readValue(client.extractMessage(response).get("content").asText(), Map.class);
        } catch (Exception e) {
            return Map.of("score", 0, "reason", "Failed to parse judge response: " + e.getMessage());
        }
    }
}
