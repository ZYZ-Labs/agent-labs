# Lab 05: Memory Systems

## Objectives

- Implement short-term memory with a sliding context window.
- Add long-term memory with a vector database (Chroma) and embeddings.
- Retrieve relevant past messages before answering.

## Run

```bash
# Start Chroma if you want long-term memory
docker compose up -d chroma

cd python
pip install -r requirements.txt
python main.py
```

## Expected Output

The agent remembers facts across turns and retrieves relevant notes when asked.
