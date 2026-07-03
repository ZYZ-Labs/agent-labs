import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.util.*;

public class Main {

    public static void main(String[] args) throws Exception {
        OpenAiClient client = null;
        try {
            client = new OpenAiClient();
        } catch (IllegalStateException exc) {
            System.out.println("LLM client disabled: " + exc.getMessage());
        }

        WorkflowEngine engine = new WorkflowEngine();
        engine.addTask(new Task("fetch", ctx -> fetchData()));
        engine.addTask(new Task("sentiment", ctx -> analyzeSentiment(client, ctx), List.of("fetch")));
        engine.addTask(new Task("quality", ctx -> flakyQualityCheck(ctx), List.of("sentiment"), 5));
        engine.addTask(new Task("summary", ctx -> summarize(client, ctx), List.of("fetch", "sentiment")));

        System.out.println("Starting DAG workflow...");
        Map<String, Object> results = engine.run();
        System.out.println("\nFinal results:");
        System.out.println(new ObjectMapper().writerWithDefaultPrettyPrinter().writeValueAsString(results));
    }

    private static Map<String, String> fetchData() {
        return Map.of(
                "title", "AI Agent Engineering",
                "content", "Workflow orchestration is essential for reliable agent systems."
        );
    }

    private static String analyzeSentiment(OpenAiClient client, Map<String, Object> ctx) throws Exception {
        @SuppressWarnings("unchecked")
        Map<String, String> fetch = (Map<String, String>) ctx.get("fetch");
        String text = fetch.get("content");
        if (client == null) {
            System.out.println("No LLM available; using deterministic sentiment fallback");
            return "positive";
        }
        ObjectMapper mapper = new ObjectMapper();
        ObjectNode payload = mapper.createObjectNode();
        ArrayNode messages = mapper.createArrayNode();
        ObjectNode system = messages.addObject();
        system.put("role", "system");
        system.put("content", "Classify sentiment as exactly one word: positive, negative, or neutral.");
        ObjectNode user = messages.addObject();
        user.put("role", "user");
        user.put("content", text);
        payload.set("messages", messages);
        payload.put("temperature", 0.0);
        payload.put("max_tokens", 10);

        JsonNode response = client.chatCompletion(payload);
        return client.extractMessage(response).get("content").asText().strip().toLowerCase();
    }

    private static FlakyChecker flakyChecker = new FlakyChecker();

    private static String flakyQualityCheck(Map<String, Object> ctx) {
        String sentiment = (String) ctx.get("sentiment");
        return flakyChecker.check(sentiment);
    }

    private static String summarize(OpenAiClient client, Map<String, Object> ctx) throws Exception {
        @SuppressWarnings("unchecked")
        Map<String, String> fetch = (Map<String, String>) ctx.get("fetch");
        String sentiment = (String) ctx.get("sentiment");
        if (client == null) {
            System.out.println("No LLM available; using deterministic summary fallback");
            return "Summary: '" + fetch.get("title") + "' has " + sentiment + " sentiment.";
        }
        String prompt = "Title: " + fetch.get("title") + "\n"
                + "Content: " + fetch.get("content") + "\n"
                + "Sentiment: " + sentiment + "\n"
                + "Write a one-sentence summary.";
        ObjectMapper mapper = new ObjectMapper();
        ObjectNode payload = mapper.createObjectNode();
        ArrayNode messages = mapper.createArrayNode();
        ObjectNode user = messages.addObject();
        user.put("role", "user");
        user.put("content", prompt);
        payload.set("messages", messages);
        payload.put("temperature", 0.0);
        payload.put("max_tokens", 60);

        JsonNode response = client.chatCompletion(payload);
        return client.extractMessage(response).get("content").asText().strip();
    }

    @FunctionalInterface
    interface TaskFunc {
        Object run(Map<String, Object> ctx) throws Exception;
    }

    static class Task {
        final String name;
        final TaskFunc func;
        final List<String> deps;
        final int retries;
        Object result;
        Exception error;

        Task(String name, TaskFunc func) {
            this(name, func, List.of(), 2);
        }

        Task(String name, TaskFunc func, List<String> deps) {
            this(name, func, deps, 2);
        }

        Task(String name, TaskFunc func, List<String> deps, int retries) {
            this.name = name;
            this.func = func;
            this.deps = deps != null ? deps : List.of();
            this.retries = retries;
        }
    }

    static class WorkflowEngine {
        private final Map<String, Task> tasks = new LinkedHashMap<>();

        void addTask(Task task) {
            tasks.put(task.name, task);
        }

        Map<String, Object> run() throws Exception {
            List<String> order = topologicalSort();
            System.out.println("Execution order: " + order);
            for (String name : order) {
                Task task = tasks.get(name);
                Map<String, Object> depResults = new HashMap<>();
                for (String dep : task.deps) {
                    depResults.put(dep, tasks.get(dep).result);
                }
                for (int attempt = 1; attempt <= task.retries; attempt++) {
                    try {
                        System.out.println("Running task '" + task.name + "' (attempt " + attempt + "/" + task.retries + ")");
                        task.result = task.func.run(depResults);
                        task.error = null;
                        break;
                    } catch (Exception exc) {
                        task.error = exc;
                        System.out.println("Task '" + task.name + "' attempt " + attempt + " failed: " + exc.getMessage());
                        if (attempt == task.retries) {
                            System.err.println("Task '" + task.name + "' exhausted retries");
                            throw exc;
                        }
                        Thread.sleep(500L * attempt);
                    }
                }
                System.out.println("Task '" + task.name + "' completed");
            }
            Map<String, Object> results = new LinkedHashMap<>();
            for (Task task : tasks.values()) {
                results.put(task.name, task.result);
            }
            return results;
        }

        private List<String> topologicalSort() {
            Map<String, Integer> inDegree = new HashMap<>();
            Map<String, List<String>> dependents = new HashMap<>();
            for (String name : tasks.keySet()) {
                inDegree.put(name, 0);
                dependents.put(name, new ArrayList<>());
            }
            for (Task task : tasks.values()) {
                for (String dep : task.deps) {
                    if (!tasks.containsKey(dep)) {
                        throw new IllegalArgumentException("Task " + task.name + " depends on unknown task " + dep);
                    }
                    inDegree.put(task.name, inDegree.get(task.name) + 1);
                    dependents.get(dep).add(task.name);
                }
            }
            List<String> queue = new ArrayList<>();
            for (Map.Entry<String, Integer> entry : inDegree.entrySet()) {
                if (entry.getValue() == 0) {
                    queue.add(entry.getKey());
                }
            }
            List<String> ordered = new ArrayList<>();
            while (!queue.isEmpty()) {
                String current = queue.remove(0);
                ordered.add(current);
                for (String dependent : dependents.get(current)) {
                    inDegree.put(dependent, inDegree.get(dependent) - 1);
                    if (inDegree.get(dependent) == 0) {
                        queue.add(dependent);
                    }
                }
            }
            if (ordered.size() != tasks.size()) {
                throw new IllegalArgumentException("Cycle detected in task dependencies");
            }
            return ordered;
        }
    }

    static class FlakyChecker {
        private int callCount = 0;

        String check(String sentiment) {
            callCount++;
            if (callCount < 3) {
                throw new RuntimeException("Quality check service unavailable (attempt " + callCount + ")");
            }
            return "quality_ok (" + sentiment + ")";
        }
    }
}
