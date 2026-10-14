import os

from PIL import Image

from .register import register


@register("image.resize")
def resize_handler(payload: dict):
    image_path = payload["imagePath"]
    width = payload["width"]
    height = payload["height"]

    try:
        with Image.open(image_path) as img:
            resized_img = img.resize((width, height))
            # keep the original extension, whatever it is, so the original file is never overwritten
            base, ext = os.path.splitext(image_path)
            outputPath = f"{base}_resized{ext}"
            resized_img.save(outputPath)
        return {"outputPath": outputPath, "error": None}, "done"
    except Exception as err:
        return {"outputPath": None, "error": str(err)}, "fail"
