import photos from "./catalogPhotos.json";

export const catalogPhotos = photos;
const byName = new Map(photos.map((photo) => [photo.name, photo]));
export function catalogPhoto(name: string) {
  return byName.get(name);
}
