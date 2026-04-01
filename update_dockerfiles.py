import os

repo_dir = "/Users/gprins/Code/domain-os"
target_line = "COPY ./internal ./internal\n"
new_line = target_line + "COPY ./pkg ./pkg\n"

count = 0
for root, dirs, files in os.walk(repo_dir):
    if 'vendor' in dirs:
        dirs.remove('vendor')
    for filename in files:
        if filename.startswith("Dockerfile"):
            filepath = os.path.join(root, filename)
            with open(filepath, 'r', encoding='utf-8') as f:
                content = f.read()
            if target_line in content and "COPY ./pkg ./pkg" not in content:
                new_content = content.replace(target_line, new_line)
                with open(filepath, 'w', encoding='utf-8') as f:
                    f.write(new_content)
                count += 1
                print(f"Updated {filepath}")
print(f"Updated {count} Dockerfiles.")
