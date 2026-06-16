import os

repo_dir = "/Users/gprins/Code/domain-os"
old_str = "github.com/onasunnymorning/domain-os/internal/domain"
new_str = "github.com/onasunnymorning/domain-os/pkg/domain"

count = 0
for root, dirs, files in os.walk(repo_dir):
    for filename in files:
        if filename.endswith(".go"):
            filepath = os.path.join(root, filename)
            with open(filepath, 'r', encoding='utf-8') as f:
                content = f.read()
            if old_str in content:
                new_content = content.replace(old_str, new_str)
                with open(filepath, 'w', encoding='utf-8') as f:
                    f.write(new_content)
                count += 1
print(f"Updated {count} files.")
