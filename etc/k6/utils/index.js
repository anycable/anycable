export function uuidv4() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    let r = Math.random() * 16 | 0, v = c === 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
}

export function randomIntBetween(min, max) { // min and max included
  return Math.floor(Math.random() * (max - min + 1) + min);
}

export function randomItem(arrayOfItems){
  return arrayOfItems[Math.floor(Math.random() * arrayOfItems.length)];
}

export function randomString(length) {
  const charset = 'abcdefghijklmnopqrstuvwxyz';
  let res = '';
  while (length--) res += charset[Math.random() * charset.length | 0];
  return res;
}

export function findBetween(content, left, right) {
  let start = content.indexOf(left);
  if (start === -1) {
    return '';
  }
  start += left.length;
  const end = content.indexOf(right, start);
  if (end === -1) {
    return '';
  }
  return content.substring(start, end);
}
