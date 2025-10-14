albumd
======

A simple, file-based, photo album sharing system. Intended to be a self-hosted replacement for cloud-based solutions like Google Photos. Generates thumbnails automatically, and requires no JS.

Album links are obfuscated, so that providing the link to one album doesn't give the public the ability to guess other albums, without requiring logins or user management.

## Running

Easiest way is to run as a container, mounting the path where each of your albums (directories) are. Sample docker-compose;

```
version: '2'

services:
  albumd:
    image: knetic/albumd:v1.2025-10-15
    container_name: albumd
    restart: unless-stopped
    environment:
      ALBUMD_USERNAME: admin
      ALBUMD_PASSWORD: adminPassword
      ALBUMD_SALT: somethingRandom
    volumes:
      - /some/path/to/somewhere:/usr/share/albumd
```

Each directory under `/usr/share/albumd` is treated as an album.

## Obfuscation

Album names (directory names) are hashed and salted with scrypt, using the `ALBUMD_SALT`. This makes brute-forcing valid album names expensive, so that bad actors can't find any album they don't have a link to.

However, this means that you also don't know the link. You have to request the endpoint `/find/<albumName>` to be redirected to the public link. The endpoint requires basic auth, with the `ALBUMD_USERNAME` / `ALBUMD_PASSWORD` as creds.

## Titling

At the root level of an album directory, any `.name` file will be read as a text file, and will be used as the header text of the album.
If that file doesn't exist, the name of the album directory is used. But it's recommended to have a friendly title, since the hashed album name comes from the directory name.

Similarly, any individual picture can have a description.

```
albums
|-- 1990-05-06_someAlbum
|------ .name          // <--- the contents of this are the title of the album
|------ image0.png
|------ image0.png.txt // <--- the contents of this are the description for image0
|------ image1.png     // <--- no description for these
|------ image2.png
```
