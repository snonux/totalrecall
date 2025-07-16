# Custom Image Prompt Feature Test

## Summary
Successfully implemented the custom image prompt feature for the TotalRecall GUI application. The feature allows users to:

1. **Enter custom prompts**: A text area is displayed next to the image where users can specify their own prompt for image generation
2. **Auto-populate prompts**: When left empty, the app automatically generates an educational prompt
3. **Display used prompts**: After image generation, the actual prompt used is displayed in the text area
4. **Preserve prompts on navigation**: When navigating to existing cards, the prompt is loaded from attribution files

## Implementation Details

### Files Modified:
1. **internal/gui/app.go**:
   - Added `imagePromptEntry` field to Application struct
   - Updated UI layout to include the prompt text area
   - Modified image generation calls to use custom prompts

2. **internal/gui/generator.go**:
   - Split `generateImages` into two functions
   - Added `generateImagesWithPrompt` to handle custom prompts
   - Updated to display used prompts in the UI after generation

3. **internal/image/search.go**:
   - Added `CustomPrompt` field to `SearchOptions` struct

4. **internal/image/openai.go**:
   - Modified to use custom prompts when provided
   - Added `GetLastPrompt()` method to retrieve the used prompt

5. **internal/image/download.go**:
   - Added `DownloadBestMatchWithOptions` and `DownloadMultipleWithOptions` methods

6. **internal/gui/navigation.go**:
   - Added logic to load prompts from attribution files when navigating

### How It Works:
1. User can enter a custom prompt in the text area next to the image
2. When generating/regenerating images, the custom prompt is used if provided
3. If no custom prompt is entered, the app generates an educational prompt automatically
4. The actual prompt used is displayed in the text area after generation
5. Prompts are saved in attribution files and loaded when navigating to existing cards

## Testing
To test the feature:
1. Run the GUI: `./totalrecall gui`
2. Enter a Bulgarian word
3. Try generating with and without custom prompts
4. Navigate between cards to verify prompt loading