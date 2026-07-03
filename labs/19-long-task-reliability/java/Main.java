import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.*;
import java.util.concurrent.*;

public class Main {

    private static final Path CHECKPOINT_DIR = Paths.get("..", ".checkpoints");

    public static void main(String[] args) throws Exception {
        OpenAiClient client = null;
        try {
            client = new OpenAiClient();
        } catch (IllegalStateException exc) {
            System.out.println("LLM client disabled: " + exc.getMessage());
        }

        String inputData = "reliable agent engineering";
        LongTask task = new LongTask(inputData, client);
        System.out.println("Starting long-running task...");
        System.out.println("Idempotency key: " + task.idempotencyKey);
        Map<String, Object> result = task.run();
        System.out.println("\nFinal result:");
        System.out.println(new ObjectMapper().writerWithDefaultPrettyPrinter().writeValueAsString(result));

        System.out.println("\nRe-running with the same idempotency key...");
        LongTask task2 = new LongTask(inputData, client);
        Map<String, Object> result2 = task2.run();
        System.out.println(new ObjectMapper().writerWithDefaultPrettyPrinter().writeValueAsString(result2));
    }

    static class CheckpointStore {
        private final Path path;

        CheckpointStore(String key) {
            this.path = CHECKPOINT_DIR.resolve(key + ".json");
        }

        @SuppressWarnings("unchecked")
        Map<String, Object> load() {
            if (!Files.exists(path)) {
                return new HashMap<>();
            }
            try {
                return new ObjectMapper().readValue(path.toFile(), Map.class);
            } catch (Exception exc) {
                return new HashMap<>();
            }
        }

        void save(Map<String, Object> state) throws Exception {
            Files.createDirectories(path.getParent());
            Path tmp = path.resolveSibling(path.getFileName().toString() + ".tmp");
            new ObjectMapper().writerWithDefaultPrettyPrinter().writeValue(tmp.toFile(), state);
            Files.move(tmp, path, java.nio.file.StandardCopyOption.REPLACE_EXISTING);
        }
    }

    static class LongTask {
        final String inputData;
        final OpenAiClient client;
        final String idempotencyKey;
        final CheckpointStore store;
        final Map<String, Object> state;
        int flakyAttempts = 0;

        LongTask(String inputData, OpenAiClient client) {
            this.inputData = inputData;
            this.client = client;
            this.idempotencyKey = makeKey(inputData);
            this.store = new CheckpointStore(this.idempotencyKey);
            this.state = this.store.load();
        }

        private String makeKey(String input) {
            return "task-" + UUID.nameUUIDFromBytes(input.getBytes());
        }

        private boolean isComplete() {
            return "completed".equals(state.get("status")) && idempotencyKey.equals(state.get("idempotency_key"));
        }

        private Object runStep(String name, Callable<Object> step, double timeout, int maxRetries) throws Exception {
            Map<String, Object> completedSteps = (Map<String, Object>) state.getOrDefault("completed_steps", new HashMap<String, Object>());
            if (Boolean.TRUE.equals(completedSteps.get(name))) {
                System.out.println("Step '" + name + "' already completed; skipping");
                Map<String, Object> results = (Map<String, Object>) state.getOrDefault("results", new HashMap<String, Object>());
                return results.get(name);
            }

            System.out.println("Executing step '" + name + "'");
            Exception lastError = null;
            for (int attempt = 1; attempt <= maxRetries; attempt++) {
                try {
                    Object result = runWithTimeout(step, timeout);
                    completedSteps.put(name, true);
                    state.put("completed_steps", completedSteps);
                    Map<String, Object> results = (Map<String, Object>) state.getOrDefault("results", new HashMap<String, Object>());
                    results.put(name, result);
                    state.put("results", results);
                    state.put("last_step", name);
                    store.save(state);
                    System.out.println("Step '" + name + "' succeeded");
                    return result;
                } catch (Exception exc) {
                    lastError = exc;
                    System.out.println("Step '" + name + "' attempt " + attempt + " failed: " + exc.getMessage());
                    if (attempt < maxRetries) {
                        double delay = exponentialBackoff(attempt);
                        System.out.printf("Retrying step '%s' in %.2fs%n", name, delay);
                        Thread.sleep((long) (delay * 1000));
                    }
                }
            }
            System.out.println("Step '" + name + "' exhausted retries");
            throw lastError;
        }

        private String stepFetchData() throws Exception {
            System.out.println("Fetching data for: " + inputData);
            if (client != null) {
                ObjectMapper mapper = new ObjectMapper();
                ObjectNode payload = mapper.createObjectNode();
                ArrayNode messages = mapper.createArrayNode();
                ObjectNode user = messages.addObject();
                user.put("role", "user");
                user.put("content", "Summarize '" + inputData + "' in one sentence.");
                payload.set("messages", messages);
                payload.put("max_tokens", 50);

                JsonNode response = client.chatCompletion(payload);
                return client.extractMessage(response).get("content").asText().strip();
            }
            return "Mock summary for '" + inputData + "'.";
        }

        private String stepProcessData(String fetched) {
            flakyAttempts++;
            if (flakyAttempts < 3) {
                throw new RuntimeException("Processing service busy (attempt " + flakyAttempts + ")");
            }
            return "processed(" + fetched + ")";
        }

        private String stepNotify(String processed) {
            return "notification_sent(" + processed + ")";
        }

        Map<String, Object> run() throws Exception {
            if (isComplete()) {
                System.out.println("Task already completed for key " + idempotencyKey + "; returning cached result");
                return Map.of("status", "completed", "results", state.get("results"));
            }

            state.putIfAbsent("idempotency_key", idempotencyKey);
            state.putIfAbsent("status", "running");
            state.putIfAbsent("completed_steps", new HashMap<String, Object>());
            state.putIfAbsent("results", new HashMap<String, Object>());

            String fetched = (String) runStep("fetch", this::stepFetchData, 5.0, 3);
            String processed = (String) runStep("process", () -> stepProcessData(fetched), 5.0, 3);
            String notified = (String) runStep("notify", () -> stepNotify(processed), 5.0, 3);

            state.put("status", "completed");
            store.save(state);
            System.out.println("Task completed successfully");
            return Map.of("status", "completed", "results", state.get("results"));
        }
    }

    private static Object runWithTimeout(Callable<Object> func, double timeoutSeconds) throws Exception {
        ExecutorService executor = Executors.newSingleThreadExecutor();
        try {
            Future<Object> future = executor.submit(func);
            return future.get((long) (timeoutSeconds * 1000), TimeUnit.MILLISECONDS);
        } catch (TimeoutException exc) {
            throw new RuntimeException("Step timed out after " + timeoutSeconds + "s", exc);
        } finally {
            executor.shutdownNow();
        }
    }

    private static double exponentialBackoff(int attempt) {
        double delay = Math.min(1.0 * Math.pow(2, attempt - 1), 30.0);
        double jitter = delay * 0.1 * (attempt % 2 == 0 ? 1 : -1);
        return delay + jitter;
    }
}
