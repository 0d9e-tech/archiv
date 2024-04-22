# directory view thumbnails

## Bytes per pixel

I want to load screen full of thumbnails in 1 second.

Let's assume that we have a screen size `(1920x1080=)2073600` pixels and our UI is
optimal and fills the entire screen with thumbnails.

Our storage is very slow. The upload speed can be as low as 2 MiB/s. But our
user count is also very low, let's say <30. Let's say that at most 4 users are
loading images at a time. That gives us a budget of 500 KiB/s per user. If we
want to fill the screen with images, each screen must have at most 500 KiB of
data.

`(512*1024)/(1920*1080) = 0.25` bytes per pixel

## Thumbnail size

Averaging google photos web view and my gallery app view, there are at most 40
images per screen. Assuming screen size 1920x1080 that gives every thumbnail
`((1920x1080)/40=) 51840` (277x277) pixels to fill.

## Jpeg quality comparison

I have taken pictures from the last 0d9e meeting. There is 32 images. Original
pictures sumed up together have 91M pixels and 23MiB. I have converted them all
to different jpeg qualities to get a sample bytes per pixel of each jpeg quality
using this script:

```
for q in `seq 10 10 100`; do for f in `find . -type f`; do; mkdir ../$q -p; convert $f -quality $q ../$q/$f; done; done
```

without changing the image size. Result:

```
1.9M  ./10
2.8M  ./20
3.8M  ./30
4.7M  ./40
5.6M  ./50
6.7M  ./60
8.4M  ./70
11.5M ./80
20.1M ./90
41.6M ./100
23.0M ./orig
```

Interestingly, quality 100 is bigger than the originals

```
(10)   1900000/91680000=0.02 bytes per pixel
(20)   2800000/91680000=0.03 bytes per pixel
(30)   3800000/91680000=0.04 bytes per pixel
(40)   4700000/91680000=0.05 bytes per pixel
(50)   5600000/91680000=0.06 bytes per pixel
(60)   6700000/91680000=0.07 bytes per pixel
(70)   8400000/91680000=0.09 bytes per pixel
(80)  11500000/91680000=0.12 bytes per pixel
(90)  20100000/91680000=0.22 bytes per pixel
(100) 41600000/91680000=0.45 bytes per pixel
(oig) 23000000/91680000=0.25 bytes per pixel
```

From the previous calculation we have 0.5 bytes per pixel so the 80 quality
seems fine and gives us some safety margins

# full screen thumbnail

## Motivation

My phone can generate images up to 6 MiB fat. Loading this over our budgeted
500KiB/s would take 12 seconds.

If we want to (1) keep original images and (2) have a single image preview
screen in the UI, we must generate thumbnails for this view.

## size and quality

These thumbnails should again use the jpeg quality 80 and be around
`(1920*1080=)2073600` pixels while keeping the ratio. That gives us
`(0.12*2073600=)248 KiB` per screen.

0.5 seconds to load in this screen might still be too slow. Lowering the quality
to 60 `(0.07*2073600=) 145KiB` would load in 0.3 seconds which is imo good enough
for sure.

Alternatively we can compress images on upload. Loading a 4080x3072 (what my
phone generates) picture @q=80 would take `(((4080*3072)*0.25)/(500*1024)=)
6.12` seconds, which is imo unusable

# Thumbnails storage imact

For every image there would be a small and a big thumbnail.

Big thumbnail adds a constant 145 KiB per image.

Small thumbnails add `(((1920x1080)/40)*0.25=) 13 KiB` per image

That is ~150KiB spent on thumbnails per image. probably fine.

