from PIL import Image
from .register import register


@register("image.resize")
def resize_handler(payload: dict):
    image_path = payload["imagePath"]
    width = payload["width"]
    height = payload["height"]

    with Image.open(image_path) as img:
        resized_img = img.resize((width, height))
        outputPath = image_path.replace(".jpg", "_resized.jpg")
        resized_img.save(outputPath)
    return {"outputPath": outputPath}
