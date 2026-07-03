import com.agentlabs.OpenAiClient;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;

import java.util.regex.Matcher;
import java.util.regex.Pattern;

public class Main {

    private static final String SYSTEM_PROMPT = """
            You are a helpful assistant that solves problems step by step.
            You must follow this format exactly:

            Thought: describe your reasoning
            Action: tool_name(arg1, arg2, ...)
            Observation: the result of the action (provided by the system)
            ...
            Final Answer: the final answer

            Available tools:
            - calculator(expression: str) - evaluates a Python arithmetic expression safely
            - finish(answer: str) - use when you have the final answer
            """;

    private static final Pattern ACTION_PATTERN = Pattern.compile("Action:\\s*(\\w+)\\((.*)\\)");

    public static void main(String[] args) throws Exception {
        OpenAiClient client = new OpenAiClient();
        String question = "What is (128 + 256) * 2 - 100?";
        System.out.println("Question: " + question);
        String answer = runReact(client, question);
        System.out.println("\nFinal Answer: " + answer);
    }

    private static String runReact(OpenAiClient client, String question) throws Exception {
        ObjectMapper mapper = new ObjectMapper();
        ArrayNode messages = mapper.createArrayNode();

        ObjectNode systemMessage = messages.addObject();
        systemMessage.put("role", "system");
        systemMessage.put("content", SYSTEM_PROMPT);

        ObjectNode userMessage = messages.addObject();
        userMessage.put("role", "user");
        userMessage.put("content", question);

        for (int step = 0; step < 10; step++) {
            ObjectNode payload = mapper.createObjectNode();
            payload.set("messages", messages);
            payload.put("temperature", 0.0);
            payload.put("max_tokens", 200);

            JsonNode response = client.chatCompletion(payload);
            String text = client.extractMessage(response).get("content").asText();

            System.out.println("\n--- Step " + (step + 1) + " ---");
            System.out.println(text);

            if (text.contains("Final Answer:")) {
                return text.split("Final Answer:", 2)[1].trim();
            }

            String observation;
            Action action = parseAction(text);
            if (action == null) {
                observation = "Observation: I did not understand the action. Please use 'Action: tool_name(args)'.";
            } else if ("calculator".equals(action.name) && !action.args.isEmpty()) {
                String result = calculator(action.args.get(0));
                observation = "Observation: " + result;
            } else if ("finish".equals(action.name) && !action.args.isEmpty()) {
                return action.args.get(0);
            } else {
                observation = "Observation: unknown tool '" + action.name + "'";
            }

            System.out.println(observation);

            ObjectNode assistantMessage = messages.addObject();
            assistantMessage.put("role", "assistant");
            assistantMessage.put("content", text);

            ObjectNode observationMessage = messages.addObject();
            observationMessage.put("role", "user");
            observationMessage.put("content", observation);
        }

        return "Reached max steps without final answer.";
    }

    private static Action parseAction(String text) {
        Matcher matcher = ACTION_PATTERN.matcher(text);
        if (!matcher.find()) {
            return null;
        }
        String name = matcher.group(1);
        String argsStr = matcher.group(2);
        java.util.List<String> args = new java.util.ArrayList<>();
        for (String part : argsStr.split(",")) {
            String trimmed = part.trim();
            if (trimmed.isEmpty()) {
                continue;
            }
            trimmed = trimmed.replace("^\"", "").replace("\"$", "").replace("^'", "").replace("'$", "");
            args.add(trimmed);
        }
        return new Action(name, args);
    }

    private static String calculator(String expression) {
        String allowed = "0123456789+-*/(). ";
        for (char c : expression.toCharArray()) {
            if (allowed.indexOf(c) < 0) {
                return "Error: invalid characters";
            }
        }
        try {
            double value = evaluateExpression(new Tokenizer(expression));
            if (Math.floor(value) == value && !Double.isInfinite(value)) {
                return Long.toString((long) value);
            }
            return Double.toString(value);
        } catch (Exception exc) {
            return "Error: " + exc.getMessage();
        }
    }

    private static class Action {
        final String name;
        final java.util.List<String> args;

        Action(String name, java.util.List<String> args) {
            this.name = name;
            this.args = args;
        }
    }

    private static class Tokenizer {
        private final String s;
        private int pos = 0;

        Tokenizer(String s) {
            this.s = s;
        }

        boolean hasNext() {
            skipWhitespace();
            return pos < s.length();
        }

        char peek() {
            skipWhitespace();
            return s.charAt(pos);
        }

        char next() {
            skipWhitespace();
            return s.charAt(pos++);
        }

        void skipWhitespace() {
            while (pos < s.length() && Character.isWhitespace(s.charAt(pos))) {
                pos++;
            }
        }
    }

    private static double evaluateExpression(Tokenizer t) {
        double value = evaluateTerm(t);
        while (t.hasNext()) {
            char op = t.peek();
            if (op == '+' || op == '-') {
                t.next();
                double rhs = evaluateTerm(t);
                value = (op == '+') ? value + rhs : value - rhs;
            } else {
                break;
            }
        }
        return value;
    }

    private static double evaluateTerm(Tokenizer t) {
        double value = evaluateFactor(t);
        while (t.hasNext()) {
            char op = t.peek();
            if (op == '*' || op == '/') {
                t.next();
                double rhs = evaluateFactor(t);
                value = (op == '*') ? value * rhs : value / rhs;
            } else {
                break;
            }
        }
        return value;
    }

    private static double evaluateFactor(Tokenizer t) {
        char ch = t.peek();
        if (ch == '(') {
            t.next();
            double value = evaluateExpression(t);
            if (t.hasNext() && t.peek() == ')') {
                t.next();
            }
            return value;
        }
        return parseNumber(t);
    }

    private static double parseNumber(Tokenizer t) {
        StringBuilder sb = new StringBuilder();
        while (t.hasNext()) {
            char ch = t.peek();
            if (Character.isDigit(ch) || ch == '.') {
                sb.append(t.next());
            } else {
                break;
            }
        }
        if (sb.isEmpty()) {
            throw new IllegalArgumentException("expected number at position " + t.pos);
        }
        return Double.parseDouble(sb.toString());
    }
}
