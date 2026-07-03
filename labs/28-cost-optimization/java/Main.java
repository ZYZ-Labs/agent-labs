import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.util.*;

public class Main {
    private static final Map<String, double[]> MODEL_PRICING = Map.of(
        "gpt-4o", new double[]{5.0, 15.0},
        "gpt-4o-mini", new double[]{0.15, 0.60},
        "gpt-3.5-turbo", new double[]{0.50, 1.50}
    );
    private static final ObjectMapper mapper = new ObjectMapper();

    public static void main(String[] args) throws Exception {
        OpenAiClient client = new OpenAiClient();

        System.out.println("Cost comparison for 1000 input + 500 output tokens:");
        for (Map.Entry<String, double[]> entry : MODEL_PRICING.entrySet()) {
            double cost = estimateCost(entry.getKey(), 1000, 500);
            System.out.printf("  %s: $%.6f%n", entry.getKey(), cost);
        }

        String[] tasks = {
            "Summarize this paragraph in one sentence.",
            "Generate an architecture design doc for a payment gateway.",
            "Refactor this Python function to use async IO."
        };
        System.out.println("\nRouting decisions:");
        for (String task : tasks) {
            String model = chooseModel(task);
            System.out.printf("  [%s] %s%n", model, task);
        }

        String userMessage = "What is the capital of France?";
        System.out.println("\nCached prompt example:\nUser: " + userMessage);
        Map<String, Object> result = runCachedPromptAgent(client, userMessage);
        System.out.println(mapper.writerWithDefaultPrettyPrinter().writeValueAsString(result));
    }

    private static double estimateCost(String model, int promptTokens, int completionTokens) {
        double[] pricing = MODEL_PRICING.getOrDefault(model, new double[]{0.0, 0.0});
        return (promptTokens * pricing[0] + completionTokens * pricing[1]) / 1_000_000.0;
    }

    private static String chooseModel(String taskDescription) {
        String desc = taskDescription.toLowerCase();
        List<String> complexSignals = List.of("architecture", "design doc", "refactor", "complex", "multistep", "review");
        return complexSignals.stream().anyMatch(desc::contains) ? "gpt-4o" : "gpt-4o-mini";
    }

    private static int countTokens(String text) {
        return Math.max(1, (int) Math.ceil(text.length() / 4.0));
    }

    private static Map<String, Object> runCachedPromptAgent(OpenAiClient client, String userMessage) throws Exception {
        String systemPrefix = "You are a concise coding assistant. Always answer in one sentence.";
        int fullTokens = countTokens(systemPrefix + "\n" + userMessage);
        int userTokens = countTokens(userMessage);

        ObjectNode payload = mapper.createObjectNode();
        ArrayNode messages = payload.putArray("messages");
        ObjectNode sys = messages.addObject();
        sys.put("role", "system");
        sys.put("content", systemPrefix);
        ObjectNode usr = messages.addObject();
        usr.put("role", "user");
        usr.put("content", userMessage);
        payload.put("max_tokens", 200);
        payload.put("temperature", 0.0);

        ObjectNode response = client.chatCompletion(payload);
        String content = client.extractMessage(response).has("content")
            ? client.extractMessage(response).get("content").asText() : "";
        int completionTokens = response.has("usage") && response.get("usage").has("completion_tokens")
            ? response.get("usage").get("completion_tokens").asInt() : countTokens(content);

        return Map.of(
            "model", "gpt-4o-mini",
            "userMessage", userMessage,
            "fullPromptTokens", fullTokens,
            "newInputTokens", userTokens,
            "completionTokens", completionTokens,
            "estimatedCostCachedUsd", estimateCost("gpt-4o-mini", userTokens, completionTokens),
            "estimatedCostUncachedUsd", estimateCost("gpt-4o-mini", fullTokens, completionTokens),
            "response", content
        );
    }
}
