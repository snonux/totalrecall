# GPT-4o Audio Support Note

## Current Status

The standard OpenAI Text-to-Speech API (`/v1/audio/speech`) currently supports only:
- `tts-1` - Standard quality
- `tts-1-hd` - High definition quality

## GPT-4o Audio Capabilities

According to OpenAI documentation, GPT-4o models have audio capabilities, but these work differently:

1. **Realtime API**: GPT-4o audio generation might be part of the new Realtime API, which uses WebSockets for bidirectional audio streaming.

2. **Chat Completions with Audio**: GPT-4o might support audio output through the chat completions API with special modality parameters, but this requires different request/response handling than the standard TTS API.

3. **Model Names**: Models like `gpt-4o-audio-preview` or `gpt-4o-mini` with audio capabilities might not be compatible with the standard TTS endpoint.

## Experimental Usage

You can try experimental model names with the `--openai-model` flag:
```bash
./totalrecall "word" --openai-model gpt-4o-audio-preview
```

However, this will likely result in a 404 error as these models require different API endpoints.

## Future Implementation

To properly support GPT-4o audio generation, we would need to:
1. Implement support for the Realtime API (WebSocket-based)
2. Or implement the chat completions API with audio modalities
3. Handle different request/response formats for audio data

For now, stick with `tts-1` or `tts-1-hd` for reliable audio generation.