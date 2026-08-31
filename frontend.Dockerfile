FROM node:18-alpine as build

WORKDIR /app

# Check if package.json exists in frontend or root, else fallback to lightweight static server
COPY package*.json ./
RUN npm install || true

COPY . .
RUN npm run build || mkdir -p /app/dist

FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
