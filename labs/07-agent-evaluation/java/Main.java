import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

public class Main {

    private static final List<Map<String, Object>> TEST_CASES = List.of(
            Map.of(
                    "input", "What port range is safe for user services?",
                    "expected_keywords", List.of("1024", "65535"),
                    "reference", "User services should use ports from 1024 to 65535."
            ),
            Map.of(
                    "input", "Explain idempotency in one sentence.",
                    "expected_keywords", List.of("same", "multiple", "result"),
                    "reference", "Idempotency means calling an operation multiple times produces the same result."
            )
    );

    public static void main(String[] args) throws Exception {
        OpenAiClient client = new OpenAiClient();
        ObjectMapper mapper = new ObjectMapper();

        List<Map<String, Object>> results = evaluateAgent(client, mapper);

        long passed = results.stream().filter(r -> (Boolean) r.get("rule_passed")).count();
        int total = results.size();
        double avgScore = results.stream()
                .mapToDouble(r -> r.get("judge_score") instanceof Number n ? n.doubleValue() : 0.0)
                .average().orElse(0.0);

        System.out.println(mapper.writerWithDefaultPrettyPrinter().writeValueAsString(results));
        System.out.println("\nRule checks passed: " + passed + "/" + total);
        System.out.printf("Average judge score: %.1f/10%n", avgScore);
    }

    private static List<Map<String, Object>> evaluateAgent(OpenAiClient client, ObjectMapper mapper) throws Exception {
        List<Map<String, Object>> results = new ArrayList<>();
        for (Map<String, Object> testCase : TEST_CASES) {
            String answer = simpleAgent(client, mapper, (String) testCase.get("input"));
            Map<String, Object> ruleResult = ruleCheck(answer, (List<String>) testCase.get("expected_keywords"));
            Map<String, Object> judgeResult = llmJudge(client, mapper, answer, (String) testCase.get("reference"));

            results.add(Map.of(
                    "input", testCase.get("input"),
                    "answer", answer,
                    "rule_passed", ruleResult.get("passed"),
                    "missing_keywords", ruleResult.get("missing"),
                    "judge_score", judgeResult.getOrDefault("score", null),
                    "judge_reason", judgeResult.getOrDefault("reason", "")
            ));
        }
        return results;
    }

    private static Map<String, Object> ruleCheck(String answer, List<String> keywords) {
        List<String> missing = new ArrayList<>();
        String lower = answer.toLowerCase();
        for (String kw : keywords) {
            if (!lower.contains(kw.toLowerCase())) {
                missing.add(kw);
            }
        }
        return Map.of("passed", missing.isEmpty(), "missing", missing);
    }

    private static Map<String, Object> llmJudge(OpenAiClient client, ObjectMapper mapper, String answer, String reference) throws Exception {
        String prompt = "Rate how well the following answer matches the reference answer.\nAnswer: " + answer
                + "\nReference: " + reference
                + "\nRespond with JSON only: {\"score\": 1-10, \"reason\": \"...\"}";

        ObjectNode payload = mapper.createObjectNode();
        ArrayNode messages = mapper.createArrayNode();
        ObjectNode user = messages.addObject();
        user.put("role", "user");
        user.put("content", prompt);
        payload.set("messages", messages);
        ObjectNode responseFormat = mapper.createObjectNode();
        responseFormat.put("type", "json_object");
        payload.set("response_format", responseFormat);
        payload.put("max_tokens", 200);
        payload.put("temperature", 0.0);

        JsonNode response = client.chatCompletion(payload);
        String content = client.extractMessage(response).get("content").asText();
        return mapper.readValue(content, Map.class);
    }

    private static String simpleAgent(OpenAiClient client, ObjectMapper mapper, String question) throws Exception {
        ObjectNode payload = mapper.createObjectNode();
        ArrayNode messages = mapper.createArrayNode();
        ObjectNode user = messages.addObject();
        user.put("role", "user");
        user.put("content", question);
        payload.set("messages", messages);
        payload.put("max_tokens", 200);
        payload.put("temperature", 0.0);

        JsonNode response = client.chatCompletion(payload);
        return client.extractMessage(response).get("content").asText();
    }
}
