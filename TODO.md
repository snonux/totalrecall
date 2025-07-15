# TODO's

## Completed
1.  [x] Implement OpenAI DALL-E image generation for flashcards
    - [x] Create OpenAI image provider implementing ImageSearcher interface
    - [x] Add configuration flags for DALL-E model, size, quality, and style
    - [x] Implement caching mechanism to avoid regenerating identical images
    - [x] Create educational prompt generation for language learning
    - [x] Add OpenAI provider to image download workflow
    - [x] Update documentation with examples and configuration

## In Progress / Remaining
1.  [ ] Write unit tests for OpenAI image provider
2.  [ ] Add cost estimation warnings in output (show estimated API costs)
3.  [ ] Test with common Bulgarian words (ябълка, котка, куче, хляб)
4.  [ ] Consider adding batch image generation for cost optimization
5.  [ ] Add image style presets for different learning contexts (e.g., children, adults)
6.  [ ] Implement fallback from OpenAI to other providers on failure
