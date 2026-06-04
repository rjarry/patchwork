from django.db import migrations, models
import django.db.models.deletion


class Migration(migrations.Migration):
    dependencies = [
        ('patchwork', '0048_series_dependencies'),
    ]

    operations = [
        migrations.AddField(
            model_name='series',
            name='previous_series',
            field=models.ForeignKey(
                blank=True,
                null=True,
                on_delete=django.db.models.deletion.SET_NULL,
                related_name='next_series',
                to='patchwork.series',
            ),
        ),
        migrations.AddField(
            model_name='project',
            name='auto_supersede',
            field=models.BooleanField(
                default=False,
                help_text=(
                    'Automatically mark patches of previous series versions '
                    'as superseded when a new version is received.'
                ),
            ),
        ),
    ]
