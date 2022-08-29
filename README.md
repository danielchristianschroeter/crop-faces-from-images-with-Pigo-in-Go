# crop-faces-from-images-with-Pigo-in-Go

This Go implementation read all .jpg, .jpeg and .png images from the src directory, detect all faces with the [Pigo face detection library](https://github.com/esimov/pigo), crop all found faces and store them with a uniqe file name crc hash into the dst directory.

## Usage

```
$ git clone https://github.com/danielchristianschroeter/crop-faces-from-images-with-Pigo-in-Go
$ cd Crop-Faces-from-Images-with-Pigo-in-Go
# Cleanup demonstration images in src and dst directory
# Copy any images into src directory
$ go run .
```

# Credits
Example group image in src directory is from https://unsplash.com/photos/Z9FZQMwCPpk
